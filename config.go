// 配置管理模块
// 包含系统配置、AI角色管理、AI模型管理、统计信息等功能
package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// ========== 系统配置 ==========

// getConfig 获取配置项
func getConfig(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")

	if key == "" {
		respondError(w, http.StatusBadRequest, "key参数不能为空")
		return
	}

	var config Config
	query := "SELECT `key`, value, updated_at FROM configs WHERE `key` = ?"
	err := db.QueryRow(query, key).Scan(&config.Key, &config.Value, &config.UpdatedAt)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusOK, map[string]string{"value": ""})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"value": config.Value})
}

// saveConfig 保存配置项
// 将配置项保存到数据库，如果已存在则更新
func saveConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求")
		return
	}

	query := "INSERT INTO configs (`key`, value, updated_at) VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE value = ?, updated_at = NOW()"

	_, err := db.Exec(query, req.Key, req.Value, req.Value)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "保存成功"})
}

func getSystemSettings(w http.ResponseWriter, r *http.Request) {
	keys := []string{
		"DB_HOST",
		"DB_PORT",
		"DB_NAME",
		"DB_USER",
		"DB_PASSWORD",
		"LISTEN_ADDR",
		"PORT",
		"RELAY_AI_BASE_URL",
		"RELAY_AI_API_KEY",
		"RELAY_AI_MODEL",
		"RELAY_AI_FILE_MODEL",
		"ai_prompt",
		"AI_PROVIDER",
		"CTYUN_MODEL",
		"access_password",
		"company_auto_fetch_time",
		"auto_exclude_days",
		"auto_delete_days",
		"AUTO_AI_ANALYSIS_ENABLED",
		"WECHAT_HIGH_SCORE_THRESHOLD",
		"WECHAT_HOOK_URL",
		"WECHAT_NOTICE_BASE_URL",
		"WECHAT_HIGH_SCORE_ROOM",
		"WECHAT_DEFAULT_ROOM",
	}

	defaults := map[string]string{
		"DB_HOST":                     getEnv("DB_HOST", "localhost"),
		"DB_PORT":                     getEnv("DB_PORT", "3306"),
		"DB_NAME":                     getEnv("DB_NAME", ""),
		"DB_USER":                     getEnv("DB_USER", ""),
		"DB_PASSWORD":                 getEnv("DB_PASSWORD", ""),
		"LISTEN_ADDR":                 getEnv("LISTEN_ADDR", "0.0.0.0"),
		"PORT":                        getEnv("PORT", "3000"),
		"RELAY_AI_BASE_URL":           getEnv("RELAY_AI_BASE_URL", defaultRelayAIBaseURL),
		"RELAY_AI_API_KEY":            getEnv("RELAY_AI_API_KEY", defaultRelayAIAPIKey),
		"RELAY_AI_MODEL":              getEnv("RELAY_AI_MODEL", defaultRelayAIModel),
		"RELAY_AI_FILE_MODEL":         getEnv("RELAY_AI_FILE_MODEL", defaultRelayAIFileModel),
		"AI_PROVIDER":                 "ctyun",
		"CTYUN_MODEL":                 "TEXT_DEEPSEEK_V4",
		"auto_exclude_days":           "3",
		"auto_delete_days":            "0",
		"AUTO_AI_ANALYSIS_ENABLED":    "true",
		"WECHAT_HOOK_URL":             getEnv("WECHAT_HOOK_URL", ""),
		"WECHAT_NOTICE_BASE_URL":      getEnv("WECHAT_NOTICE_BASE_URL", ""),
		"WECHAT_HIGH_SCORE_THRESHOLD": getEnv("WECHAT_HIGH_SCORE_THRESHOLD", "90"),
		"WECHAT_DEFAULT_ROOM":         getEnv("WECHAT_DEFAULT_ROOM", ""),
		"WECHAT_HIGH_SCORE_ROOM": firstNonEmpty(
			getEnv("WECHAT_HIGH_SCORE_ROOM", ""),
			getEnv("WECHAT_DEFAULT_ROOM", ""),
		),
	}

	result := map[string]string{}
	for _, key := range keys {
		var value sql.NullString
		err := db.QueryRow("SELECT value FROM configs WHERE `key` = ? LIMIT 1", key).Scan(&value)
		if err != nil && err != sql.ErrNoRows {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if value.Valid && strings.TrimSpace(value.String) != "" {
			result[key] = value.String
		} else {
			result[key] = defaults[key]
		}
	}

	result["ai_logic"] = "AI 提供者可切换 relay（中转）或 ctyun（直连）。项目分析默认读取数据库 ai_prompt（设置页可编辑）；AI 自动任务可选择 AI 角色，并可单独覆盖任务提示词。"
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

func ensureAIScheduledTaskPromptColumn() error {
	_, err := db.Exec("ALTER TABLE ai_scheduled_tasks ADD COLUMN prompt_override TEXT NULL AFTER question")
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}
	return nil
}

func ensureCompanyAutoFetchConfig() error {
	defaults := map[string]string{
		"company_auto_fetch_time": "02:00",
	}
	for key, value := range defaults {
		if _, err := db.Exec("INSERT INTO configs (`key`, value, updated_at) VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE `key` = `key`", key, value); err != nil {
			return err
		}
	}
	return nil
}

func ensureSystemConfigsInitialized() error {
	defaults := map[string]string{
		"DB_HOST":                       getEnv("DB_HOST", "localhost"),
		"DB_PORT":                       getEnv("DB_PORT", "3306"),
		"DB_NAME":                       getEnv("DB_NAME", ""),
		"DB_USER":                       getEnv("DB_USER", ""),
		"DB_PASSWORD":                   getEnv("DB_PASSWORD", ""),
		"LISTEN_ADDR":                   getEnv("LISTEN_ADDR", "0.0.0.0"),
		"PORT":                          getEnv("PORT", "3000"),
		"RELAY_AI_BASE_URL":             getEnv("RELAY_AI_BASE_URL", defaultRelayAIBaseURL),
		"RELAY_AI_API_KEY":              getEnv("RELAY_AI_API_KEY", defaultRelayAIAPIKey),
		"RELAY_AI_AUTH_TOKEN":           getEnv("RELAY_AI_AUTH_TOKEN", ""),
		"RELAY_AI_COOKIE":               getEnv("RELAY_AI_COOKIE", ""),
		"RELAY_AI_MODEL":                getEnv("RELAY_AI_MODEL", defaultRelayAIModel),
		"RELAY_AI_FILE_MODEL":           getEnv("RELAY_AI_FILE_MODEL", defaultRelayAIFileModel),
		"AI_PROVIDER":                   "ctyun",
		"CTYUN_MODEL":                   "TEXT_DEEPSEEK_V4",
		"WECHAT_HOOK_URL":               getEnv("WECHAT_HOOK_URL", ""),
		"WECHAT_NOTICE_BASE_URL":        getEnv("WECHAT_NOTICE_BASE_URL", ""),
		"WECHAT_DEFAULT_ROOM":           getEnv("WECHAT_DEFAULT_ROOM", ""),
		"WECHAT_HIGH_SCORE_ROOM":        firstNonEmpty(getEnv("WECHAT_HIGH_SCORE_ROOM", ""), getEnv("WECHAT_DEFAULT_ROOM", "")),
		"WECHAT_HIGH_SCORE_THRESHOLD":   getEnv("WECHAT_HIGH_SCORE_THRESHOLD", "90"),
		"ai_prompt":                     fetchSystemPrompt(),
		"auto_exclude_old_bids_enabled": "true",
		"auto_exclude_days":             "3",
		"auto_delete_days":              "0",
		"AUTO_AI_ANALYSIS_ENABLED":      "true",
	}

	// repairOnEmpty 列出"必须非空"的关键 key：当数据库里这些 key 的值是空字符串/NULL 时，
	// 启动会自动用 defaults 修复，避免前端误写空字符串导致的开关被关、微信群发空等问题。
	// 注意：用户可能故意留空的字段（auth_token、cookie、access_password）不在这里。
	repairOnEmpty := map[string]bool{
		"AUTO_AI_ANALYSIS_ENABLED":      true,
		"auto_exclude_days":             true,
		"auto_delete_days":              true,
		"auto_exclude_old_bids_enabled": true,
		"WECHAT_HOOK_URL":               true,
		"WECHAT_NOTICE_BASE_URL":        true,
		"WECHAT_DEFAULT_ROOM":           true,
		"WECHAT_HIGH_SCORE_ROOM":        true,
		"WECHAT_HIGH_SCORE_THRESHOLD":   true,
		"RELAY_AI_BASE_URL":             true,
		"RELAY_AI_API_KEY":              true,
		"RELAY_AI_MODEL":                true,
		"RELAY_AI_FILE_MODEL":           true,
		"DB_HOST":                       true,
		"DB_PORT":                       true,
		"DB_NAME":                       true,
		"DB_USER":                       true,
		"LISTEN_ADDR":                   true,
		"PORT":                          true,
	}

	repaired := 0
	for key, value := range defaults {
		// 1) 不存在则插入；存在则保留（绝大多数 key 走这条）
		if _, err := db.Exec(
			"INSERT INTO configs (`key`, value, updated_at) VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE `key` = `key`",
			key, value,
		); err != nil {
			return err
		}

		// 2) 关键 key 自动修复：值是 NULL 或空字符串时，写回默认值
		if repairOnEmpty[key] && strings.TrimSpace(value) != "" {
			res, err := db.Exec(
				"UPDATE configs SET value = ?, updated_at = NOW() WHERE `key` = ? AND (value IS NULL OR TRIM(value) = '')",
				value, key,
			)
			if err != nil {
				return err
			}
			if affected, _ := res.RowsAffected(); affected > 0 {
				log.Printf("[启动自动修复] 配置项 %s 的值为空，已恢复默认值: %s", key, value)
				repaired++
			}
		}
	}
	if repaired > 0 {
		log.Printf("[启动自动修复] 共修复 %d 项被误写空的关键配置", repaired)
	}
	return nil
}

func getCompanyAutoFetchConfig(w http.ResponseWriter, r *http.Request) {
	var value sql.NullString
	err := db.QueryRow("SELECT value FROM configs WHERE `key` = 'company_auto_fetch_time' LIMIT 1").Scan(&value)
	if err != nil && err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := "02:00"
	if value.Valid && strings.TrimSpace(value.String) != "" {
		result = value.String
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]string{"company_auto_fetch_time": result},
	})
}

func saveCompanyAutoFetchConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CompanyAutoFetchTime string `json:"company_auto_fetch_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求")
		return
	}
	if strings.TrimSpace(req.CompanyAutoFetchTime) == "" {
		respondError(w, http.StatusBadRequest, "自动获取时间不能为空")
		return
	}
	if len(req.CompanyAutoFetchTime) != 5 || req.CompanyAutoFetchTime[2] != ':' {
		respondError(w, http.StatusBadRequest, "时间格式必须为 HH:mm")
		return
	}
	hour, err1 := strconv.Atoi(req.CompanyAutoFetchTime[:2])
	minute, err2 := strconv.Atoi(req.CompanyAutoFetchTime[3:])
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		respondError(w, http.StatusBadRequest, "时间格式必须为 HH:mm")
		return
	}
	_, err := db.Exec("INSERT INTO configs (`key`, value, updated_at) VALUES ('company_auto_fetch_time', ?, NOW()) ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = NOW()", req.CompanyAutoFetchTime)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"message": "保存成功"})
}

func saveAutoExcludeSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AutoExcludeDays string `json:"auto_exclude_days"`
		AutoDeleteDays  string `json:"auto_delete_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求")
		return
	}
	days, err := strconv.Atoi(strings.TrimSpace(req.AutoExcludeDays))
	if err != nil || days < 1 || days > 365 {
		respondError(w, http.StatusBadRequest, "自动排除天数必须在 1 到 365 之间")
		return
	}
	if _, err := db.Exec("INSERT INTO configs (`key`, value, updated_at) VALUES ('auto_exclude_old_bids_enabled', 'true', NOW()) ON DUPLICATE KEY UPDATE value = 'true', updated_at = NOW()"); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := db.Exec("INSERT INTO configs (`key`, value, updated_at) VALUES ('auto_exclude_days', ?, NOW()) ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = NOW()", strconv.Itoa(days)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	deleteDays := 0
	if strings.TrimSpace(req.AutoDeleteDays) != "" {
		parsedDeleteDays, err := strconv.Atoi(strings.TrimSpace(req.AutoDeleteDays))
		if err != nil || parsedDeleteDays < 0 || parsedDeleteDays > 3650 {
			respondError(w, http.StatusBadRequest, "自动删除天数必须在 0 到 3650 之间")
			return
		}
		deleteDays = parsedDeleteDays
	}
	if _, err := db.Exec("INSERT INTO configs (`key`, value, updated_at) VALUES ('auto_delete_days', ?, NOW()) ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = NOW()", strconv.Itoa(deleteDays)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	affected, err := excludeOldBidsByDays(days)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "保存成功，已立即执行一次自动排除",
		"data": map[string]interface{}{
			"affected":         affected,
			"auto_delete_days": deleteDays,
		},
	})
}

func verifyAccessPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求")
		return
	}
	var value sql.NullString
	err := db.QueryRow("SELECT value FROM configs WHERE `key` = 'access_password' LIMIT 1").Scan(&value)
	if err != nil && err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	saved := ""
	if value.Valid {
		saved = value.String
	}
	if strings.TrimSpace(saved) == "" || req.Password == saved {
		respondJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]bool{"success": true}})
		return
	}
	respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "访问密码错误"})
}

func isAccessPasswordProtectedPath(path string) bool {
	switch path {
	case "/api/settings/system",
		"/api/config",
		"/config",
		"/api/getConfig",
		"/getConfig",
		"/api/saveConfig",
		"/saveConfig",
		"/api/settings/auto-exclude",
		"/api/executeSQL",
		"/executeSQL":
		return true
	default:
		return false
	}
}

func accessPasswordMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAccessPasswordProtectedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		var value sql.NullString
		err := db.QueryRow("SELECT value FROM configs WHERE `key` = 'access_password' LIMIT 1").Scan(&value)
		if err != nil && err != sql.ErrNoRows {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !value.Valid || strings.TrimSpace(value.String) == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-Access-Password") != value.String {
			respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "访问密码错误"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ========== AI 角色管理 ==========

// initAIRolesTable 初始化 AI 角色表
// 如果表不存在则创建，如果表为空则插入默认角色数据
func initAIRolesTable() {
	// 创建 ai_roles 表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS ai_roles (
		id INT AUTO_INCREMENT PRIMARY KEY,
		role_key VARCHAR(50) NOT NULL UNIQUE COMMENT '角色唯一标识',
		role_name VARCHAR(100) NOT NULL COMMENT '角色名称',
		description TEXT COMMENT '角色描述',
		prompt TEXT NOT NULL COMMENT '系统提示词',
		is_default BOOLEAN DEFAULT FALSE COMMENT '是否默认角色',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_role_key (role_key)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Printf("创建 ai_roles 表失败: %v", err)
		return
	}

	// 检查是否已有数据
	var count int
	db.QueryRow("SELECT COUNT(*) FROM ai_roles").Scan(&count)

	if count == 0 {
		// 插入默认角色
		defaultRoles := []struct {
			key         string
			name        string
			description string
			prompt      string
			isDefault   bool
		}{
			{
				key:         "analyst",
				name:        "招标分析顾问",
				description: "专注于招标项目分析，基于公司情况评估项目匹配度",
				prompt: `你是一名专业的招标项目智能分析顾问，负责评估某物流运输企业是否应参与特定招标项目。

【公司信息】
- 类型：物流运输企业
- 车队规模：180辆货车
- 所在地：云南省玉溪市
- 核心业务：烟草运输（常年服务）
- 倾向业务：大宗商品运输、烟草行业运输
- 不承接业务：危险品、建筑材料、装修工程
- 服务能力：大型车队运输、长途运输、专业物流服务
- 服务区域：云南省

【分析职责】
1. 分析招标项目的关键特征和趋势
2. 提取并总结共同关键词
3. 基于公司情况提供专业的投标建议
4. 评估项目与公司业务的匹配度

请用专业、简洁、有条理的方式回答问题。`,
				isDefault: true,
			},
			{
				key:         "assistant",
				name:        "通用助手",
				description: "通用对话助手，回答各类问题",
				prompt: `你是一个智能助手，帮助用户理解和分析招标信息。

【公司信息】
- 类型：物流运输企业
- 车队规模：180辆货车
- 所在地：云南省玉溪市
- 核心业务：烟草运输（常年服务）
- 倾向业务：大宗商品运输、烟草行业运输
- 不承接业务：危险品、建筑材料、装修工程
- 服务能力：大型车队运输、长途运输、专业物流服务
- 服务区域：云南省

【工作方式】
1. 以友好、易懂的方式回答问题
2. 提供清晰的解释和建议
3. 如有需要，可以询问更多细节以提供更好的帮助

请保持专业但亲切的语气。`,
				isDefault: false,
			},
			{
				key:         "expert",
				name:        "行业专家",
				description: "物流运输行业专家，提供专业建议和行业洞察",
				prompt: `你是一位资深的物流运输行业专家，拥有多年的行业经验和深厚的专业知识。

【公司信息】
- 类型：物流运输企业
- 车队规模：180辆货车
- 所在地：云南省玉溪市
- 核心业务：烟草运输（常年服务）
- 倾向业务：大宗商品运输、烟草行业运输
- 不承接业务：危险品、建筑材料、装修工程
- 服务能力：大型车队运输、长途运输、专业物流服务
- 服务区域：云南省

【专业能力】
1. 深入理解物流运输行业的运作模式和发展趋势
2. 能够识别行业机遇和潜在风险
3. 提供基于行业最佳实践的专业建议
4. 分析市场竞争格局和企业定位

请以行业专家的视角，提供深入、专业的分析和建议。`,
				isDefault: false,
			},
			{
				key:         "consultant",
				name:        "投标顾问",
				description: "投标策略顾问，提供投标决策和竞争分析",
				prompt: `你是一位经验丰富的投标策略顾问，专注于帮助企业制定有效的投标决策。

【公司信息】
- 类型：物流运输企业
- 车队规模：180辆货车
- 所在地：云南省玉溪市
- 核心业务：烟草运输（常年服务）
- 倾向业务：大宗商品运输、烟草行业运输
- 不承接业务：危险品、建筑材料、装修工程
- 服务能力：大型车队运输、长途运输、专业物流服务
- 服务区域：云南省

【咨询重点】
1. 评估投标机会的可行性和竞争优势
2. 分析项目要求与企业能力的匹配程度
3. 提供投标准备和资源配置建议
4. 识别潜在风险和应对策略
5. 建议是否参与投标及优先级排序

请从投标顾问的角度，提供战略性、可操作的建议。`,
				isDefault: false,
			},
		}

		insertSQL := `INSERT INTO ai_roles (role_key, role_name, description, prompt, is_default) VALUES (?, ?, ?, ?, ?)`
		for _, role := range defaultRoles {
			if _, err := db.Exec(insertSQL, role.key, role.name, role.description, role.prompt, role.isDefault); err != nil {
				log.Printf("插入默认角色 %s 失败: %v", role.name, err)
			}
		}
		log.Println("✓ AI 角色表初始化完成，已插入默认角色")
	}
}

// getAIRoles 获取所有 AI 角色
func getAIRoles(w http.ResponseWriter, r *http.Request) {
	query := "SELECT id, role_key, role_name, description, prompt, is_default, created_at, updated_at FROM ai_roles ORDER BY is_default DESC, id ASC"
	rows, err := db.Query(query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var roles []AIRole
	for rows.Next() {
		var role AIRole
		err := rows.Scan(&role.ID, &role.RoleKey, &role.RoleName, &role.Description,
			&role.Prompt, &role.IsDefault, &role.CreatedAt, &role.UpdatedAt)
		if err != nil {
			log.Printf("扫描角色数据失败: %v", err)
			continue
		}
		roles = append(roles, role)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       roles,
	})
}

// createAIRole 创建新 AI 角色
func createAIRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoleKey     string `json:"role_key"`
		RoleName    string `json:"role_name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		IsDefault   bool   `json:"is_default"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求: "+err.Error())
		return
	}

	// 验证必填字段
	if req.RoleKey == "" || req.RoleName == "" || req.Prompt == "" {
		respondError(w, http.StatusBadRequest, "角色标识、角色名称和提示词不能为空")
		return
	}

	// 如果设置为默认，先取消其他默认角色
	if req.IsDefault {
		db.Exec("UPDATE ai_roles SET is_default = FALSE")
	}

	query := "INSERT INTO ai_roles (role_key, role_name, description, prompt, is_default) VALUES (?, ?, ?, ?, ?)"
	result, err := db.Exec(query, req.RoleKey, req.RoleName, req.Description, req.Prompt, req.IsDefault)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			respondError(w, http.StatusBadRequest, "角色标识已存在")
		} else {
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	id, _ := result.LastInsertId()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "创建成功",
		"data":       map[string]int64{"id": id},
	})
}

// updateAIRole 更新 AI 角色
func updateAIRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req struct {
		RoleKey     string `json:"role_key"`
		RoleName    string `json:"role_name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		IsDefault   bool   `json:"is_default"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求: "+err.Error())
		return
	}

	// 验证必填字段
	if req.RoleKey == "" || req.RoleName == "" || req.Prompt == "" {
		respondError(w, http.StatusBadRequest, "角色标识、角色名称和提示词不能为空")
		return
	}

	// 如果设置为默认，先取消其他默认角色
	if req.IsDefault {
		db.Exec("UPDATE ai_roles SET is_default = FALSE WHERE id != ?", id)
	}

	query := "UPDATE ai_roles SET role_key = ?, role_name = ?, description = ?, prompt = ?, is_default = ? WHERE id = ?"
	result, err := db.Exec(query, req.RoleKey, req.RoleName, req.Description, req.Prompt, req.IsDefault, id)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			respondError(w, http.StatusBadRequest, "角色标识已存在")
		} else {
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondError(w, http.StatusNotFound, "角色不存在")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "更新成功",
	})
}

// deleteAIRole 删除 AI 角色
func deleteAIRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// 检查是否为默认角色
	var isDefault bool
	err := db.QueryRow("SELECT is_default FROM ai_roles WHERE id = ?", id).Scan(&isDefault)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "角色不存在")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if isDefault {
		respondError(w, http.StatusBadRequest, "不能删除默认角色")
		return
	}

	query := "DELETE FROM ai_roles WHERE id = ?"
	_, err = db.Exec(query, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "删除成功",
	})
}

// ========== AI 模型管理 ==========

// initAIModelsTable 初始化 AI 模型表
func initAIModelsTable() {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS ai_models (
		id INT AUTO_INCREMENT PRIMARY KEY,
		model_key VARCHAR(100) NOT NULL UNIQUE COMMENT '模型唯一标识',
		model_name VARCHAR(200) NOT NULL COMMENT '模型名称',
		provider VARCHAR(50) NOT NULL COMMENT '提供商',
		description TEXT COMMENT '模型描述',
		input_price DECIMAL(10, 2) DEFAULT 0 COMMENT '输入价格（元/M tokens）',
		output_price DECIMAL(10, 2) DEFAULT 0 COMMENT '输出价格（元/M tokens）',
		is_default BOOLEAN DEFAULT FALSE COMMENT '是否默认模型',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_model_key (model_key),
		INDEX idx_is_default (is_default)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Printf("创建 ai_models 表失败: %v", err)
		return
	}

	// 检查是否已有数据
	var count int
	db.QueryRow("SELECT COUNT(*) FROM ai_models").Scan(&count)

	if count == 0 {
		// 插入默认模型
		defaultModels := []struct {
			key         string
			name        string
			provider    string
			description string
			inputPrice  float64
			outputPrice float64
			isDefault   bool
		}{
			{"GLM-4.5-Flash", "GLM-4.5-Flash", "智谱", "快速响应的GLM模型，适合日常对话", 1.58, 6.32, true},
			{"deepseek-v3.2-exp", "DeepSeek-v3.2-exp", "DeepSeek", "DeepSeek 3.2 支持128K上下文", 1.58, 2.37, false},
			{"deepseek-v3.2-exp-thinking", "DeepSeek-v3.2-exp-thinking", "DeepSeek", "v3.2深度思考版本", 1.58, 2.37, false},
			{"DeepSeek-V3.1-Fast", "DeepSeek-V3.1-Fast", "DeepSeek", "DeepSeek-V3.1超级极速版，语速每秒平均150 tokens", 6.32, 18.96, false},
			{"deepseek-chat", "deepseek-chat", "DeepSeek", "2025-09-29 更新为 DeepSeek-V3.2-Exp，价格大幅下降", 2.0, 3.0, false},
			{"deepseek-reasoner", "deepseek-reasoner", "DeepSeek", "已经更新为 DeepSeek-V3.2-Exp 深度思考版", 2.0, 3.0, false},
			{"qwen-max-latest", "qwen-max-latest", "阿里云Qwen", "阿里旗舰模型，2025-11-15 降价50%", 2.4, 9.6, false},
			{"qwen-turbo-latest", "qwen-turbo-latest", "阿里云Qwen", "阿里主力模型，超高性价比", 0.2, 0.4, false},
			{"glm-4.6", "GLM-4.6", "智谱", "智谱最新旗舰模型，355B参数", 1.58, 6.32, false},
			{"glm-4.6-thinking", "GLM-4.6-thinking", "智谱", "GLM-4.6 思考版本", 2.0, 8.0, false},
			{"glm-4.5v", "GLM-4.5v", "智谱", "视觉理解模型，支持图片问答、OCR等", 1.58, 4.74, false},
			{"glm-4-flash", "GLM-4-Flash", "智谱", "免费调用的智谱模型", 0.0, 0.0, false},
		}

		insertSQL := `INSERT INTO ai_models (model_key, model_name, provider, description, input_price, output_price, is_default) VALUES (?, ?, ?, ?, ?, ?, ?)`
		for _, model := range defaultModels {
			if _, err := db.Exec(insertSQL, model.key, model.name, model.provider, model.description, model.inputPrice, model.outputPrice, model.isDefault); err != nil {
				log.Printf("插入默认模型 %s 失败: %v", model.name, err)
			}
		}
		log.Println("✓ AI 模型表初始化完成，已插入默认模型")
	}

	// 始终同步中转 AI 实际支持的模型清单（已存在则跳过，不会覆盖用户改过的描述/价格）
	syncRelayAIModels()
}

// syncRelayAIModels 把 /docs 接口列出的中转模型同步到 ai_models 表。
// 已存在的 model_key 会被忽略，不破坏用户自定义的描述和价格。
func syncRelayAIModels() {
	relayModels := []struct {
		key         string
		name        string
		provider    string
		description string
		inputPrice  float64
		outputPrice float64
	}{
		{"TEXT_DEEPSEEK_V4", "DeepSeek-V4", "DeepSeek", "中转默认模型，强制开启联网搜索+深度思考，支持文件提问", 0, 0},
		{"TEXT_A14", "GLM-5", "智谱", "智谱 GLM-5，强制开启联网搜索+深度思考，支持文件提问", 0, 0},
		{"TEXT_A22", "Qwen3.5-Plus", "阿里云Qwen", "千问 3.5 Plus，强制开启联网搜索+深度思考，支持文件提问", 0, 0},
		{"TEXT_A13", "Qwen3:30B", "阿里云Qwen", "千问 3 30B，强制开启联网搜索+深度思考，支持文件提问", 0, 0},
		{"TEXT_A8", "DeepSeek-V3.2", "DeepSeek", "DeepSeek V3.2，强制开启联网搜索+深度思考，支持文件提问", 0, 0},
		{"lingxi", "WPS Lingxi", "WPS", "独立链路，文件上传走 /api/lingxi/files/upload-share，需要 Lingxi cookie", 0, 0},
	}

	const insertIgnoreSQL = `INSERT IGNORE INTO ai_models (model_key, model_name, provider, description, input_price, output_price, is_default) VALUES (?, ?, ?, ?, ?, ?, ?)`
	added := 0
	for _, m := range relayModels {
		res, err := db.Exec(insertIgnoreSQL, m.key, m.name, m.provider, m.description, m.inputPrice, m.outputPrice, false)
		if err != nil {
			log.Printf("同步中转模型 %s 失败: %v", m.key, err)
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added++
		}
	}
	if added > 0 {
		log.Printf("✓ 中转 AI 模型同步完成，新增 %d 个", added)
	}
}

// getAIModels 获取所有 AI 模型
func getAIModels(w http.ResponseWriter, r *http.Request) {
	query := "SELECT id, model_key, model_name, provider, description, input_price, output_price, is_default, created_at, updated_at FROM ai_models ORDER BY is_default DESC, created_at DESC"
	rows, err := db.Query(query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var models []AIModel
	for rows.Next() {
		var model AIModel
		err := rows.Scan(&model.ID, &model.ModelKey, &model.ModelName, &model.Provider, &model.Description, &model.InputPrice, &model.OutputPrice, &model.IsDefault, &model.CreatedAt, &model.UpdatedAt)
		if err != nil {
			log.Printf("扫描模型失败: %v", err)
			continue
		}
		models = append(models, model)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       models,
	})
}

// createAIModel 创建 AI 模型
func createAIModel(w http.ResponseWriter, r *http.Request) {
	var model AIModel
	if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求: "+err.Error())
		return
	}

	// 验证必填字段
	if model.ModelKey == "" || model.ModelName == "" || model.Provider == "" {
		respondError(w, http.StatusBadRequest, "模型标识、名称和提供商为必填项")
		return
	}

	// 检查 model_key 是否已存在
	var count int
	db.QueryRow("SELECT COUNT(*) FROM ai_models WHERE model_key = ?", model.ModelKey).Scan(&count)
	if count > 0 {
		respondError(w, http.StatusBadRequest, "模型标识已存在")
		return
	}

	// 如果设置为默认，先取消其他默认
	if model.IsDefault {
		db.Exec("UPDATE ai_models SET is_default = FALSE")
	}

	query := `INSERT INTO ai_models (model_key, model_name, provider, description, input_price, output_price, is_default) VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := db.Exec(query, model.ModelKey, model.ModelName, model.Provider, model.Description, model.InputPrice, model.OutputPrice, model.IsDefault)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	id, _ := result.LastInsertId()
	model.ID = int(id)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       model,
	})
}

// updateAIModel 更新 AI 模型
func updateAIModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var model AIModel
	if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求: "+err.Error())
		return
	}

	// 如果设置为默认，先取消其他默认
	if model.IsDefault {
		db.Exec("UPDATE ai_models SET is_default = FALSE")
	}

	query := `UPDATE ai_models SET model_key = ?, model_name = ?, provider = ?, description = ?, input_price = ?, output_price = ?, is_default = ? WHERE id = ?`
	_, err := db.Exec(query, model.ModelKey, model.ModelName, model.Provider, model.Description, model.InputPrice, model.OutputPrice, model.IsDefault, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "更新成功",
	})
}

// deleteAIModel 删除 AI 模型
func deleteAIModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// 检查是否为默认模型
	var isDefault bool
	err := db.QueryRow("SELECT is_default FROM ai_models WHERE id = ?", id).Scan(&isDefault)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "模型不存在")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if isDefault {
		respondError(w, http.StatusBadRequest, "不能删除默认模型")
		return
	}

	query := "DELETE FROM ai_models WHERE id = ?"
	_, err = db.Exec(query, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "删除成功",
	})
}

// ========== 统计信息 ==========

// getStatistics 获取统计信息
func getStatistics(w http.ResponseWriter, r *http.Request) {
	stats := Statistics{
		SourceStats: make(map[string]int),
	}

	// 总数
	var totalBids sql.NullInt64
	db.QueryRow("SELECT COUNT(*) FROM bids").Scan(&totalBids)
	if totalBids.Valid {
		stats.TotalBids = int(totalBids.Int64)
	}

	// 适合数量
	var suitableBids sql.NullInt64
	db.QueryRow("SELECT COUNT(*) FROM bids WHERE ai_suitable = 1").Scan(&suitableBids)
	if suitableBids.Valid {
		stats.SuitableBids = int(suitableBids.Int64)
	}

	// 平均分数
	var avgScore sql.NullFloat64
	db.QueryRow("SELECT AVG(ai_score) FROM bids WHERE ai_score > 0").Scan(&avgScore)
	if avgScore.Valid {
		stats.AvgScore = avgScore.Float64
	}

	// 来源统计
	rows, err := db.Query("SELECT source, COUNT(*) as count FROM bids GROUP BY source")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var source string
			var count int
			if err := rows.Scan(&source, &count); err == nil {
				stats.SourceStats[source] = count
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       stats,
	})
}

// ensureNoAIView 确保 no_ai 视图按"未AI评分 且 未排除"的口径维护。
// 已排除（status=1 / -1）的项目不应再进入批量 AI 分析队列。
func ensureNoAIView() error {
	const ddl = `
    CREATE OR REPLACE VIEW no_ai AS
    SELECT id, source
    FROM bids
    WHERE (ai_analysis IS NULL OR ai_analysis = '')
      AND (status IS NULL OR status = 0)
    ORDER BY created_at
    `
	if _, err := db.Exec(ddl); err != nil {
		return err
	}
	return nil
}
