// 主程序入口文件
// 负责初始化数据库、设置路由、启动HTTP服务器和定时任务
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// init 初始化函数，在 main 函数执行前自动调用
// 负责加载环境变量、初始化数据库连接、确保数据库表结构正确
func init() {
	// 加载环境变量
	godotenv.Load()
	// 初始化数据库连接
	initDB()
	if err := ensureOriginalPDFURLColumn(); err != nil {
		log.Printf("初始化 bids.original_pdf_url 失败: %v", err)
	}
	if err := ensureSystemConfigsInitialized(); err != nil {
		log.Printf("初始化系统配置失败: %v", err)
	}
	if err := ensureAIScheduledTaskPromptColumn(); err != nil {
		log.Printf("初始化 ai_scheduled_tasks.prompt_override 失败: %v", err)
	}
	if err := ensureCompanyAutoFetchConfig(); err != nil {
		log.Printf("初始化企业自动获取配置失败: %v", err)
	}
	if err := ensureNoAIView(); err != nil {
		log.Printf("初始化 no_ai 视图失败: %v", err)
	}
	// 确保 tracked_bids 表的结构完整（添加必要的字段）
	if err := ensureTrackedBidsEnhancements(); err != nil {
		log.Printf("初始化 tracked_bids schema 失败: %v", err)
	}
	// 用户画像 + 每日问候表
	if err := ensureMascotInsightColumn(); err != nil {
		log.Printf("初始化 bids.mascot_insight 失败: %v", err)
	}
	if err := ensureUserProfileTables(); err != nil {
		log.Printf("初始化 user_profile 表失败: %v", err)
	}
}

// main 主函数，程序的入口点
// 负责创建路由、注册API端点、设置静态文件服务、启动定时任务和HTTP服务器
func main() {
	// 创建路由
	r := mux.NewRouter()
	r.SkipClean(true)
	// 兼容所有根路径接口的 /api/ 前缀访问
	r.HandleFunc("/api/queryPotentialBidders", queryPotentialBidders).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/getExistingUUIDs", getExistingUUIDs).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/getBids", getBids).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/getExcludedBids", getExcludedBids).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/maxSerial", maxSerialHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/getRecentIds", getRecentIDsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/getTodaySerials", getTodaySerialsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/restoreBid", restoreBid).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/deleteBid", deleteBid).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/batchExclude", batchExclude).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/batchDelete", batchDelete).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/insertBid", insertBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/insertChinaBid", insertChinaBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/insertZhibiaoBid", insertZhibiaoBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/analyzeBid", analyzeBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/analyzeById", analyzeByIDHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/reanalyzeBid", reanalyzeBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/kimiai", kimiAIHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/deleteChinaData", deleteChinaDataHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/executeSQL", executeSQLHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/statistics", workerStatisticsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/autoExcludeOldBids", autoExcludeOldBidsHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/checkStatusDistribution", checkStatusDistributionHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/updateStatus", updateStatusHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/updatePdf", updatePDFHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/checkTitle", checkTitleHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/checkSerial", checkSerialHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/checkDuplicates", checkDuplicatesHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/checkIds", checkIDsHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/checkId", checkIDHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/savePotentialBidders", savePotentialBiddersHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/getPotentialBidders", getPotentialBiddersHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/saveSimilarProjects", saveSimilarProjectsHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/getSimilarProjects", getSimilarProjectsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/trackWinner", trackWinnerHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/uploadChina", uploadChinaHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/uploadChinaById", uploadChinaByIDHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/pdfProxy", pdfProxy).Methods("GET", "HEAD", "OPTIONS")
	r.HandleFunc("/api/getBidDetail", getBidDetail).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/getConfig", getConfig).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/saveConfig", saveConfig).Methods("POST", "OPTIONS")

	storageDir = getEnv("STORAGE_DIR", filepath.Join("storage", "r2"))
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		log.Fatalf("创建存储目录失败: %v", err)
	}
	uploadToken = getEnv("UPLOAD_TOKEN", "")

	// API路由
	api := r.PathPrefix("/api").Subrouter()

	// 招标相关
	api.HandleFunc("/bids", getBids).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/list", getBids).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/excluded", getExcludedBids).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/detail", getBidDetail).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/today", getTodayBiddings).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/date-range", getBiddingsByDateRange).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/search", searchBids).Methods("POST", "OPTIONS")
	api.HandleFunc("/bids/getPotentialBidders", getPotentialBiddersHandler).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/getSimilarProjects", getSimilarProjectsHandler).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/savePotentialBidders", savePotentialBiddersHandler).Methods("POST", "OPTIONS")
	api.HandleFunc("/bids/saveSimilarProjects", saveSimilarProjectsHandler).Methods("POST", "OPTIONS")
	api.HandleFunc("/bids/trackWinner", trackWinnerHandler).Methods("POST", "OPTIONS")
	api.HandleFunc("/bids/updateStatus", updateStatusHandler).Methods("POST", "OPTIONS")

	// AI 分析相关（REST 风格路径，兼容文档描述）
	api.HandleFunc("/ai/analyze", analyzeBidHandler).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/analyzeById", analyzeByIDHandler).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/reanalyze", reanalyzeBidHandler).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/batchAnalyze", batchAnalyzeHandler).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/chat", chatWithAIHandler).Methods("POST", "OPTIONS")

	// CTYun 直连聊天
	api.HandleFunc("/chat/models", ctyunChatModels).Methods("GET", "OPTIONS")
	api.HandleFunc("/chat/upload", ctyunChatUpload).Methods("POST", "OPTIONS")
	api.HandleFunc("/chat/stream", ctyunChatStream).Methods("POST", "OPTIONS")

	// 小招AI 每日问候 + 项目情报
	api.HandleFunc("/mascot/daily", mascotDailyHandler).Methods("GET", "OPTIONS")
	api.HandleFunc("/bids/insight", getBidInsight).Methods("GET", "OPTIONS")

	// UUID去重接口
	api.HandleFunc("/existingUUIDs", getExistingUUIDs).Methods("GET", "OPTIONS")

	// 配置相关
	api.HandleFunc("/config", getConfig).Methods("GET", "OPTIONS")
	api.HandleFunc("/config", saveConfig).Methods("POST", "OPTIONS")
	api.HandleFunc("/settings/system", getSystemSettings).Methods("GET", "OPTIONS")
	api.HandleFunc("/settings/auth/verify", verifyAccessPassword).Methods("POST", "OPTIONS")
	api.HandleFunc("/settings/company-auto-fetch", getCompanyAutoFetchConfig).Methods("GET", "OPTIONS")
	api.HandleFunc("/settings/company-auto-fetch", saveCompanyAutoFetchConfig).Methods("POST", "OPTIONS")
	api.HandleFunc("/settings/auto-exclude", saveAutoExcludeSettings).Methods("POST", "OPTIONS")

	// AI 角色管理
	api.HandleFunc("/ai/roles", getAIRoles).Methods("GET", "OPTIONS")
	api.HandleFunc("/ai/roles", createAIRole).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/roles/{id}", updateAIRole).Methods("PUT", "OPTIONS")
	api.HandleFunc("/ai/roles/{id}", deleteAIRole).Methods("DELETE", "OPTIONS")

	// AI 模型管理
	api.HandleFunc("/ai/models", getAIModels).Methods("GET", "OPTIONS")
	api.HandleFunc("/ai/models", createAIModel).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/models/{id}", updateAIModel).Methods("PUT", "OPTIONS")
	api.HandleFunc("/ai/models/{id}", deleteAIModel).Methods("DELETE", "OPTIONS")

	// 统计相关
	api.HandleFunc("/statistics", getStatistics).Methods("GET", "OPTIONS")

	// 文件上传（替换R2）
	api.HandleFunc("/upload", handleFileUpload).Methods("POST", "OPTIONS")
	api.Path("/files/{path:.*}").HandlerFunc(handleFileGet).Methods("GET")

	// 微信通知
	api.HandleFunc("/wechat/send", sendWechatNotification).Methods("POST", "OPTIONS")
	api.HandleFunc("/api/sendProjectToWechat", sendProjectToWechatHandler).Methods("POST", "OPTIONS")
	api.HandleFunc("/sendProjectToWechat", sendProjectToWechatHandler).Methods("POST", "OPTIONS")

	// 追踪中标人相关
	api.HandleFunc("/tracked-bids/{id}", deleteTrackedBid).Methods("DELETE", "OPTIONS")
	api.HandleFunc("/tracked-bids", getTrackedBids).Methods("GET", "OPTIONS")
	api.HandleFunc("/tracked-bids/{id}/start-fetch", startTrackedWinnerFetch).Methods("POST", "OPTIONS")
	api.HandleFunc("/tracked-bids/{id}/stop-fetch", stopTrackedWinnerFetch).Methods("POST", "OPTIONS")
	api.HandleFunc("/tracked-bids/{id}/complete", completeTrackedBid).Methods("POST", "OPTIONS")
	api.HandleFunc("/bids/exclude-all", excludeAllBids).Methods("POST", "OPTIONS")
	api.HandleFunc("/bids/manual-exclude-old", manualExcludeOldBidsHandler).Methods("POST", "OPTIONS")

	// 公司中标历史
	api.HandleFunc("/company-awards/companies", getCompanyAwardSummary).Methods("GET", "OPTIONS")
	api.HandleFunc("/company-awards/records", getCompanyAwardRecords).Methods("GET", "OPTIONS")
	api.HandleFunc("/company-awards/detail", getCompanyAwardDetail).Methods("GET", "OPTIONS")
	api.HandleFunc("/company-awards/search-realtime", searchCompanyRealtime).Methods("POST", "OPTIONS")
	api.HandleFunc("/company-awards/auto-fetch/status", getCompanyAutoFetchStatus).Methods("GET", "OPTIONS")
	api.HandleFunc("/company-awards/auto-fetch/run", runCompanyAutoFetchHandler).Methods("POST", "OPTIONS")

	// AI定时任务管理
	api.HandleFunc("/ai/tasks", getAITasks).Methods("GET", "OPTIONS")
	api.HandleFunc("/ai/tasks", createAITask).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/tasks/{id}", getAITask).Methods("GET", "OPTIONS")
	api.HandleFunc("/ai/tasks/{id}", updateAITask).Methods("PUT", "OPTIONS")
	api.HandleFunc("/ai/tasks/{id}", deleteAITask).Methods("DELETE", "OPTIONS")
	api.HandleFunc("/ai/tasks/{id}/toggle", toggleAITask).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/tasks/{id}/execute", executeAITask).Methods("POST", "OPTIONS")
	api.HandleFunc("/ai/tasks/{id}/history", getAITaskHistory).Methods("GET", "OPTIONS")

	// Worker 兼容路由
	r.HandleFunc("/queryPotentialBidders", queryPotentialBidders).Methods("GET", "OPTIONS")
	r.HandleFunc("/getExistingUUIDs", getExistingUUIDs).Methods("GET", "OPTIONS")
	r.HandleFunc("/getBids", getBids).Methods("GET", "OPTIONS")
	r.HandleFunc("/getExcludedBids", getExcludedBids).Methods("GET", "OPTIONS")
	r.HandleFunc("/maxSerial", maxSerialHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/getRecentIds", getRecentIDsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/getTodaySerials", getTodaySerialsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/restoreBid", restoreBid).Methods("POST", "OPTIONS")
	r.HandleFunc("/deleteBid", deleteBid).Methods("POST", "OPTIONS")
	r.HandleFunc("/batchExclude", batchExclude).Methods("POST", "OPTIONS")
	r.HandleFunc("/batchDelete", batchDelete).Methods("POST", "OPTIONS")
	r.HandleFunc("/insertBid", insertBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/insertChinaBid", insertChinaBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/insertZhibiaoBid", insertZhibiaoBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/analyzeBid", analyzeBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/analyzeById", analyzeByIDHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/reanalyzeBid", reanalyzeBidHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/kimiai", kimiAIHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/deleteChinaData", deleteChinaDataHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/executeSQL", executeSQLHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/statistics", workerStatisticsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/autoExcludeOldBids", autoExcludeOldBidsHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/checkStatusDistribution", checkStatusDistributionHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/updateStatus", updateStatusHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/updateStatus", updateStatusHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/updatePdf", updatePDFHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/checkTitle", checkTitleHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/checkSerial", checkSerialHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/checkDuplicates", checkDuplicatesHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/checkIds", checkIDsHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/checkId", checkIDHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/savePotentialBidders", savePotentialBiddersHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/getPotentialBidders", getPotentialBiddersHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/saveSimilarProjects", saveSimilarProjectsHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/getSimilarProjects", getSimilarProjectsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/trackWinner", trackWinnerHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/tracked-bids/{id}/start-fetch", startTrackedWinnerFetch).Methods("POST", "OPTIONS")
	r.HandleFunc("/tracked-bids/{id}/stop-fetch", stopTrackedWinnerFetch).Methods("POST", "OPTIONS")
	r.HandleFunc("/tracked-bids/{id}/complete", completeTrackedBid).Methods("POST", "OPTIONS")
	r.HandleFunc("/uploadChina", uploadChinaHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/uploadChinaById", uploadChinaByIDHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/pdfProxy", pdfProxy).Methods("GET", "HEAD", "OPTIONS")
	r.HandleFunc("/getBidDetail", getBidDetail).Methods("GET", "OPTIONS")
	r.HandleFunc("/getConfig", getConfig).Methods("GET", "OPTIONS")
	r.HandleFunc("/saveConfig", saveConfig).Methods("POST", "OPTIONS")
	r.PathPrefix("/").Methods("PUT").HandlerFunc(handleRawPutUpload)

	// 静态文件托管
	staticDir := filepath.Join("dist")

	// 本地存储文件托管 (PDF等) - 必须在其他路由之前注册，使用 MatcherFunc 确保精确匹配
	chinaDir := filepath.Join(storageDir, "china")
	chinaFileServer := http.StripPrefix("/china/", http.FileServer(http.Dir(chinaDir)))
	r.PathPrefix("/china/").MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
		// 只匹配 /china/ 开头的路径
		return strings.HasPrefix(r.URL.Path, "/china/")
	}).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// 设置正确的Content-Type
		if strings.HasSuffix(req.URL.Path, ".pdf") {
			w.Header().Set("Content-Type", "application/pdf")
		}
		chinaFileServer.ServeHTTP(w, req)
	})

	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(staticDir, "assets")))))
	r.PathPrefix("/favicon.ico").Handler(http.FileServer(http.Dir(staticDir)))
	r.PathPrefix("/robots.txt").Handler(http.FileServer(http.Dir(staticDir)))

	// 前端history路由支持：非API、非静态资源都返回index.html
	r.PathPrefix("/").MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
		// 排除已经匹配的路由
		path := r.URL.Path
		return !strings.HasPrefix(path, "/api/") &&
			!strings.HasPrefix(path, "/assets/") &&
			!strings.HasPrefix(path, "/china/") &&
			path != "/favicon.ico" &&
			path != "/robots.txt"
	}).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		indexPath := filepath.Join(staticDir, "index.html")
		http.ServeFile(w, r, indexPath)
	})

	// CORS和日志中间件
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Upload-Token", "X-Title"}),
	)

	normalizedRouter := cleanPathMiddleware(r)
	corsWrapped := corsHandler(accessPasswordMiddleware(normalizedRouter))
	loggedRouter := handlers.LoggingHandler(os.Stdout, corsWrapped)

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tryServeStoredFile(w, r) {
			return
		}
		http.NotFound(w, r)
	})

	// 初始化 CTYun 直连聊天客户端
	ctyunClient = NewCTYunClient()
	if ctyunClient != nil {
		go ctyunClient.StartAutoRenew()
		log.Println("✓ CTYun 直连聊天客户端已初始化")
	}

	// ── Graceful shutdown context ───────────────────────────────
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ── Structured logger (JSON) ────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 启动定时任务调度器
	startCronJobs(ctx)

	// 启动服务器
	port := getEnv("PORT", "3000")
	addr := getEnv("LISTEN_ADDR", "0.0.0.0") + ":" + port
	srv := &http.Server{Addr: addr, Handler: loggedRouter}
	go func() {
		slog.Info("服务器启动", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待关闭信号
	<-ctx.Done()
	slog.Info("收到关闭信号，开始优雅退出...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("服务器关闭失败", "error", err)
	}
	slog.Info("服务器已安全关闭")
}

// initDB 初始化数据库连接
// 从环境变量读取数据库配置，建立MySQL连接，设置连接池参数
func initDB() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true",
		getEnv("DB_USER", ""),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_HOST", ""),
		getEnv("DB_PORT", ""),
		getEnv("DB_NAME", ""),
	)

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	// 测试连接
	if err = db.Ping(); err != nil {
		log.Fatal("数据库ping失败:", err)
	}

	// 设置连接池 — 适配高并发 AI 分析 + 爬虫 + 前端查询
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// 预编译高频 INSERT 语句 — 消除每次插入的 SQL 解析开销
	prepareBidStatements()

	log.Println("✓ 数据库连接成功")

	// // 初始化 AI 角色表
	// initAIRolesTable()
	// // 初始化 AI 模型表
	// initAIModelsTable()
}

// ========== 定时任务相关 ==========

// startCronJobs 启动定时任务调度器
// 启动三个定时任务：
// 1. 批量AI分析任务（每10分钟执行一次）
// 2. AI定时任务检查（每1分钟检查一次用户创建的定时任务）
// 3. 获取追踪项目的中标结果（每10分钟执行一次）
func startCronJobs(ctx context.Context) {
	// 优雅退出的 sleep 封装
	sleepOrCancel := func(d time.Duration) bool {
		select {
		case <-ctx.Done():
			return true
		case <-time.After(d):
			return false
		}
	}

	// 1. 近实时执行批量AI分析 — run-then-sleep 模式杜绝任务重叠
	go func() {
		for {
			slog.Info("[CRON] 开始检查未分析项目...")
			batchAnalyzeUnprocessedBids()
			if sleepOrCancel(15 * time.Second) {
				slog.Info("[CRON] 批量AI分析收到退出信号，停止")
				return
			}
		}
	}()

	// 2. 每分钟检查并执行用户创建的AI定时任务
	go func() {
		for {
			checkAndExecuteAITasks()
			if sleepOrCancel(1 * time.Minute) {
				slog.Info("[CRON] AI定时任务收到退出信号，停止")
				return
			}
		}
	}()

	// 3. 每10分钟执行一次获取追踪项目的中标结果
	go func() {
		if sleepOrCancel(30 * time.Second) {
			return
		}
		for {
			slog.Info("[CRON] 开始执行获取中标结果任务...")
			fetchTrackedBidsWinners()
			if sleepOrCancel(10 * time.Minute) {
				slog.Info("[CRON] 中标结果获取收到退出信号，停止")
				return
			}
		}
	}()

	// 4. 每天执行一次自动排除三天前的招标
	go func() {
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		if sleepOrCancel(time.Until(nextMidnight)) {
			return
		}
		for {
			slog.Info("[CRON] 开始执行自动排除旧招标...")
			autoExcludeOldBids()
			autoDeleteOldBids()
			if sleepOrCancel(24 * time.Hour) {
				slog.Info("[CRON] 自动排除收到退出信号，停止")
				return
			}
		}
	}()

	// 5. 企业库自动获取
	go func() {
		for {
			var configured sql.NullString
			runAt := "02:00"
			if err := db.QueryRow("SELECT value FROM configs WHERE `key` = 'company_auto_fetch_time' LIMIT 1").Scan(&configured); err == nil && configured.Valid && strings.TrimSpace(configured.String) != "" {
				runAt = configured.String
			}
			now := time.Now()
			parts := strings.Split(runAt, ":")
			hour, minute := 2, 0
			if len(parts) == 2 {
				if h, err := strconv.Atoi(parts[0]); err == nil {
					hour = h
				}
				if m, err := strconv.Atoi(parts[1]); err == nil {
					minute = m
				}
			}
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
			if !nextRun.After(now) {
				nextRun = nextRun.Add(24 * time.Hour)
			}
			if sleepOrCancel(time.Until(nextRun)) {
				slog.Info("[CRON] 企业库自动获取收到退出信号，停止")
				return
			}
			slog.Info("[CRON] 开始执行企业库自动获取任务...")
			runCompanyAutoFetch()
		}
	}()

	slog.Info("[CRON] 定时任务调度器已启动",
		"batchAI", "每15秒",
		"aiTasks", "每1分钟",
		"winnerFetch", "每10分钟",
		"autoExclude", "每天凌晨",
		"companyFetch", "每天按配置时间",
	)
}
