// 数据模型和全局变量定义
// 包含所有数据结构、全局变量和常量定义
package main

import (
	"bytes"
	"database/sql"
	"log"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// ========== 全局变量 ==========

// db 全局数据库连接对象
var db *sql.DB

// storageDir 文件存储目录路径
var storageDir string

// uploadToken 文件上传验证令牌
var uploadToken string

// ── Shared connection-pooling Transport ──────────────────────────
// 全局复用，避免每次 new http.Client 导致 TIME_WAIT 堆积
var sharedTransport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:          200,
	MaxIdleConnsPerHost:   100,
	MaxConnsPerHost:       200,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// httpClient 全局HTTP客户端，设置30秒超时（用于普通短请求：上传、查询等）
var httpClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: sharedTransport,
}

// streamingAIHTTPClient 流式 AI 专用 HTTP 客户端
// 流式 AI 走 SSE，长文档分析单次响应可能持续 1~5 分钟，30s 不够
// 这里设 10 分钟兜底超时；实际更细的"无 token 输出闲置超时"在 SSE 读循环里手动控制
var streamingAIHTTPClient = &http.Client{
	Timeout:   10 * time.Minute,
	Transport: sharedTransport,
}

// longHTTPClient 60s 超时 — 下载 PDF、大文件抓取
var longHTTPClient = &http.Client{
	Timeout:   60 * time.Second,
	Transport: sharedTransport,
}

// storageProxyClient 50s 超时 — 文件代理回源
var storageProxyClient = &http.Client{
	Timeout:   50 * time.Second,
	Transport: sharedTransport,
}

// ── sync.Pool: bytes.Buffer 复用，减少 GC 压力 ────────────────
// 用于 JSON 编解码、HTTP 请求体构建等高频临时分配场景
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bufferPool.Put(buf)
}

var relayAIInvokeState = struct {
	mu         sync.Mutex
	lastInvoke time.Time
}{}
var batchAIState = struct {
	mu            sync.Mutex
	running       bool
	lastStartedAt time.Time
}{}

// batchAIHardExpiry batchAIState 最大允许运行时间，超时自动重置 running 标志
const batchAIHardExpiry = 5 * time.Minute
var companyAutoFetchState = struct {
	mu           sync.Mutex
	running      bool
	startedAt    time.Time
	finishedAt   time.Time
	totalCount   int
	successCount int
	failedCount  int
	lastMessage  string
}{}

// ctyunClient CTYun 直连聊天客户端
var ctyunClient *CTYunClient

// ── 预编译 SQL 语句 — 高频 INSERT 路径零编译开销 ──────────────
var (
	stmtInsertBid       *sql.Stmt
	stmtInsertChinaBid  *sql.Stmt
	stmtInsertZhibiaoBid *sql.Stmt
)

// ========== 常量定义 ==========

const (
	// defaultRelayAIBaseURL 默认流式中转AI服务地址
	defaultRelayAIBaseURL = ""
	// defaultRelayAIModel 默认流式中转AI模型
	defaultRelayAIModel = "TEXT_DEEPSEEK_V4"
	// defaultRelayAIFileModel 默认文件问答链路（旧模型）
	defaultRelayAIFileModel = "default"
	// defaultRelayAIAPIKey 默认流式中转AI密钥
	defaultRelayAIAPIKey = ""
)

var (
	// scriptTagPattern 匹配HTML中script标签的正则表达式
	scriptTagPattern = regexp.MustCompile(`(?is)<script[^>]*?>[\s\S]*?</script>`)
	// styleTagPattern 匹配HTML中style标签的正则表达式
	styleTagPattern = regexp.MustCompile(`(?is)<style[^>]*?>[\s\S]*?</style>`)
	// htmlCommentPattern 匹配HTML注释的正则表达式
	htmlCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)
	// newlineTagPattern 匹配换行相关HTML标签的正则表达式
	newlineTagPattern = regexp.MustCompile(`(?i)</?(p|div|br|li|tr|h[1-6])[^>]*>`)

	// htmlTagPattern 匹配所有HTML标签的正则表达式
	htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
	// whitespacePattern 匹配连续空白字符的正则表达式
	whitespacePattern = regexp.MustCompile(`\s+`)
	// thinkTagPattern 匹配AI思考过程标签的正则表达式
	thinkTagPattern = regexp.MustCompile(`(?s)<think>(.*?)</think>`)
	// candidatePattern 匹配“中标候选人”相关行
	candidatePattern = regexp.MustCompile(`(?m)(第[一二三四五六七八九十]+中标候选人|中标候选人[一二三四五六七八九十0-9]*|第一中标人|第二中标人|第三中标人)[：:]\s*([^；。.\r\n]+)`)
	// aiExcludeKeywords AI分析时需要排除的关键词列表（不匹配的项目）
	aiExcludeKeywords = []string{"危货", "危险品", "危险废物", "危废", "建筑材料", "建材", "土建", "房建", "装修工程", "装修", "拆除", "爆破", "软件开发", "网络设备", "消防器材", "安防", "监控"}
	// transportKeywords 运输相关关键词列表
	transportKeywords = []string{"运输"}
	// timeOnlyPattern 匹配时间格式（HH:mm:ss）的正则表达式
	timeOnlyPattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
)

// prepareBidStatements 预编译高频 INSERT SQL，消除每次解析开销
func prepareBidStatements() {
	var err error
	stmtInsertBid, err = db.Prepare(`INSERT INTO bids (
		id, serial, title, buyer, fetch_time,
		area, city, district, budget, bid_amount,
		buyer_person, buyer_tel, agency, agency_person, agency_tel,
		publish_time, sign_end_time, bid_end_time,
		detail, description, industry, site, original_href, pdf_url, html_url,
		source, project_code, project_name, public_type, top_type, sub_type,
		purchasing, keywords, deliver_area, deliver_city, deliver_detail,
		created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Printf("预编译 stmtInsertBid 失败: %v", err)
	}

	stmtInsertChinaBid, err = db.Prepare(`INSERT INTO bids (
		id, serial, title, buyer, fetch_time,
		area, city, district, budget, bid_amount,
		buyer_person, buyer_tel, agency, agency_person, agency_tel,
		publish_time, sign_end_time, bid_end_time,
		detail, description, industry, site, original_href, pdf_url, html_url,
		source, project_code, project_name, public_type, top_type, sub_type,
		purchasing, keywords, deliver_area, deliver_city, deliver_detail,
		created_at,
		china_bulletin_id, china_notice_media, china_bulletin_type, china_platform_name,
		china_region_code, china_bulletin_source, china_supervise_dept, china_data_source,
		china_classify_name, china_tender_agency, china_doc_get_end_time, china_bid_doc_refer_end_time,
		china_server_plat, china_trade_plat, china_data_plat, china_left_open_bid_day, china_is_new,
		china_tender_project_name, china_bid_section_name, china_notice_name,
		china_potential_bidders, china_similar_projects
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Printf("预编译 stmtInsertChinaBid 失败: %v", err)
	}

	stmtInsertZhibiaoBid, err = db.Prepare(`INSERT INTO bids (
		id, serial, title, buyer, fetch_time,
		area, budget, bid_amount,
		agency, agency_person, agency_tel,
		buyer_person, buyer_tel,
		publish_time, sign_end_time, bid_end_time, bid_open_time,
		detail, description, industry, keywords,
		original_href, pdf_url, source,
		project_code, project_name, public_type,
		created_at,
		zhibiao_notice_type, zhibiao_procurement_method,
		zhibiao_ceil_price, zhibiao_credential,
		zhibiao_supplier, zhibiao_supplier_date,
		zhibiao_project_demand, zhibiao_project_content,
		zhibiao_project_addr, zhibiao_project_node,
		zhibiao_bid_type, zhibiao_sign_start_time,
		zhibiao_plan_time, zhibiao_high_light_title
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Printf("预编译 stmtInsertZhibiaoBid 失败: %v", err)
	}
}

func ensureMascotInsightColumn() error {
	_, err := db.Exec("ALTER TABLE bids ADD COLUMN mascot_insight TEXT NULL AFTER ai_thinking_process")
	if err != nil && !regexp.MustCompile(`(?i)duplicate column`).MatchString(err.Error()) {
		return err
	}
	return nil
}

func ensureOriginalPDFURLColumn() error {
	_, err := db.Exec("ALTER TABLE bids ADD COLUMN original_pdf_url TEXT NULL AFTER pdf_url")
	if err != nil && !regexp.MustCompile(`(?i)duplicate column`).MatchString(err.Error()) {
		return err
	}
	return nil
}

// ========== 数据结构定义 ==========

// Config 配置项结构体
// 用于存储系统配置信息（如AI提示词等）
type Config struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AIRole AI角色结构体
// 定义AI分析时使用的角色配置，包括角色名称、描述和提示词
type AIRole struct {
	ID          int       `json:"id"`
	RoleKey     string    `json:"role_key"`
	RoleName    string    `json:"role_name"`
	Description string    `json:"description"`
	Prompt      string    `json:"prompt"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AIModel AI模型结构体
// 定义可用的AI模型配置，包括模型标识、名称、提供商和价格信息
type AIModel struct {
	ID          int       `json:"id"`
	ModelKey    string    `json:"model_key"`
	ModelName   string    `json:"model_name"`
	Provider    string    `json:"provider"`
	Description string    `json:"description"`
	InputPrice  float64   `json:"input_price"`
	OutputPrice float64   `json:"output_price"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Bid 招标信息结构体
// 用于存储招标项目的基本信息和AI分析结果
type Bid struct {
	ID           string         `json:"id"`
	Serial       string         `json:"serial"`
	Title        string         `json:"title"`
	Buyer        string         `json:"buyer"`
	Area         sql.NullString `json:"area"`
	City         sql.NullString `json:"city"`
	Budget       float64        `json:"budget"`
	Industry     sql.NullString `json:"industry"`
	PublishTime  int64          `json:"publish_time"`
	BidEndTime   int64          `json:"bid_end_time"`
	AIAnalysis   sql.NullString `json:"ai_analysis"`
	AISuitable   int            `json:"ai_suitable"`
	AIScore      int            `json:"ai_score"`
	AIMatchLevel sql.NullString `json:"ai_match_level"`
	AIPriority   sql.NullString `json:"ai_priority"`
	Source       string         `json:"source"`
	Status       int            `json:"status"`
	WechatSent   int            `json:"wechat_sent"`
	WechatSentAt sql.NullString `json:"wechat_sent_at"`
	CreatedAt    string         `json:"created_at"`
	Keywords     sql.NullString `json:"keywords"`
	Site         sql.NullString `json:"site"`
}

// BidRecord 招标记录结构体
// 用于从数据库读取招标详细信息，包含更多字段用于AI分析
type BidRecord struct {
	ID                     string
	Serial                 sql.NullString
	Title                  sql.NullString
	Buyer                  sql.NullString
	ProjectName            sql.NullString
	ProjectCode            sql.NullString
	Industry               sql.NullString
	Area                   sql.NullString
	City                   sql.NullString
	District               sql.NullString
	Purchasing             sql.NullString
	Keywords               sql.NullString
	Budget                 sql.NullFloat64
	Description            sql.NullString
	Detail                 sql.NullString
	PDFURL                 sql.NullString
	Source                 sql.NullString
	PublicType             sql.NullString
	TopType                sql.NullString
	SubType                sql.NullString
	DeliverArea            sql.NullString
	DeliverCity            sql.NullString
	DeliverDetail          sql.NullString
	ChinaTenderProjectName sql.NullString
	ChinaBidSectionName    sql.NullString
	ChinaNoticeName        sql.NullString
	ChinaClassifyName      sql.NullString
}

// Statistics 统计信息结构体
// 用于存储招标数据的统计信息，包括总数、匹配数、平均分数等
type Statistics struct {
	TotalBids    int            `json:"total_bids"`
	SuitableBids int            `json:"suitable_bids"`
	AvgScore     float64        `json:"avg_score"`
	SourceStats  map[string]int `json:"source_stats"`
}

// WechatPayload 微信消息载荷结构体
// 用于构建发送到微信的消息格式
type WechatPayload struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// WechatResponse 微信API响应结构体
// 用于解析微信API返回的响应
type WechatResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// AIScheduledTask AI定时任务结构体
// 定义用户创建的AI定时任务配置，包括执行时间、AI配置、数据源等
type AIScheduledTask struct {
	ID          int    `json:"id"`
	TaskName    string `json:"task_name"`
	Description string `json:"description"`

	// 执行时间配置
	CronExpression string `json:"cron_expression"`
	ScheduleTime   string `json:"schedule_time"`
	ScheduleType   string `json:"schedule_type"`

	// AI配置
	AIRole         string `json:"ai_role"`
	AIModel        string `json:"ai_model"`
	Question       string `json:"question"`
	PromptOverride string `json:"prompt_override"`

	// 数据配置
	DataSource string `json:"data_source"`
	DateFrom   string `json:"date_from"`
	DateTo     string `json:"date_to"`

	// 微信推送配置
	EnableWechat bool   `json:"enable_wechat"`
	WechatRoomID string `json:"wechat_room_id"`

	// 执行状态
	IsActive      bool   `json:"is_active"`
	LastRunAt     string `json:"last_run_at"`
	LastRunStatus string `json:"last_run_status"`
	LastRunResult string `json:"last_run_result"`
	NextRunAt     string `json:"next_run_at"`

	// 统计信息
	TotalRuns   int `json:"total_runs"`
	SuccessRuns int `json:"success_runs"`
	FailedRuns  int `json:"failed_runs"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AITaskHistory AI任务执行历史结构体
// 记录每次AI定时任务的执行结果，包括执行状态、AI响应、错误信息等
type AITaskHistory struct {
	ID         int       `json:"id"`
	TaskID     int       `json:"task_id"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Status     string    `json:"status"`

	AIRole   string `json:"ai_role"`
	AIModel  string `json:"ai_model"`
	Question string `json:"question"`

	DataCount  int    `json:"data_count"`
	AIResponse string `json:"ai_response"`
	ErrorMsg   string `json:"error_message"`

	WechatSent   bool   `json:"wechat_sent"`
	WechatResult string `json:"wechat_result"`

	CreatedAt time.Time `json:"created_at"`
}

type CompanyAwardRecord struct {
	ID            int    `json:"id"`
	BulletinID    string `json:"bulletin_id"`
	ProjectName   string `json:"project_name"`
	WinBidder     string `json:"win_bidder"`
	WinPrice      string `json:"win_price"`
	NoticeTime    string `json:"notice_time"`
	NoticeURL     string `json:"notice_url"`
	Details       string `json:"details"`
	SearchCompany string `json:"search_company"`
}

// WinnerInfo 中标信息结构体
// 用于存储从API获取的中标结果详细信息，包括中标单位、金额、相关公告链接等
type WinnerInfo struct {
	Winner              string   // 中标单位名称（最终中标人或当前主候选人）
	WinnerAmount        float64  // 中标金额（单位：元）
	TenderNoticeName    string   // 招标公告名称
	TenderNoticeURL     string   // 招标公告链接
	CandidateNoticeName string   // 中标候选人公示名称
	CandidateNoticeURL  string   // 中标候选人公示链接
	CandidatePDFURL     string   // 中标候选人公示PDF链接（本地存储路径）
	CandidateNoticeTime string   // 候选人公示发布时间
	ResultNoticeName    string   // 中标结果公告名称
	ResultNoticeURL     string   // 中标结果公告链接
	ResultPDFURL        string   // 中标结果公告PDF链接（本地存储路径）
	ResultNoticeTime    string   // 中标结果公告发布时间
	FullDetails         string   // 完整JSON详情（原始API响应）
	Candidates          []string // 所有解析出的候选人名称列表
	HasCandidate        bool     // 是否已解析到候选人信息（即使还没有最终中标结果）
	HasResult           bool     // 是否已解析到最终中标结果
}

// UnanalyzedBid 未分析的招标项目
type UnanalyzedBid struct {
	ID                string
	Source            string
	TenderProjectName string
	BidSectionName    string
	NoticeName        string
	ChinaClassifyName string
	Keywords          string
	Area              string
	City              string
	Serial            string
}

// FullBidInfo 完整的招标信息
type FullBidInfo struct {
	Budget      float64
	Site        string
	Buyer       string
	Industry    string
	PublishTime int64
	BidEndTime  int64
}

// BatchAnalysisResult AI批量分析结果
type BatchAnalysisResult struct {
	Projects []ProjectAnalysisResult `json:"projects"`
}

// ProjectAnalysisResult 单个项目分析结果
type ProjectAnalysisResult struct {
	Index           int            `json:"index"`
	Suitable        bool           `json:"suitable"`
	Score           int            `json:"score"`
	MatchLevel      string         `json:"matchLevel"`
	Priority        string         `json:"priority"`
	DimensionScores map[string]int `json:"dimensionScores"`
	Reasons         []string       `json:"reasons"`
	Advantages      []string       `json:"advantages"`
	Risks           []string       `json:"risks"`
	Recommendation  string         `json:"recommendation"`
}

// aiComputationResult AI计算结果
type aiComputationResult struct {
	ID          string
	Serial      string
	Analysis    map[string]interface{}
	AIModel     string
	Prompt      string
	RawResponse string
	Thinking    string
	AnalyzedAt  string
}
