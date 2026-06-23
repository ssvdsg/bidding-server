// AI定时任务管理模块
// 包含AI定时任务的创建、更新、删除、执行、历史记录等功能
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// ========== HTTP Handlers ==========

// getAITasks 获取所有定时任务
func getAITasks(w http.ResponseWriter, r *http.Request) {
	log.Printf("[获取任务] 开始查询所有定时任务")

	// 先检查表是否存在，并统计总记录数
	var totalCount int
	countErr := db.QueryRow("SELECT COUNT(*) FROM ai_scheduled_tasks").Scan(&totalCount)
	if countErr != nil {
		if strings.Contains(countErr.Error(), "doesn't exist") || strings.Contains(countErr.Error(), "Table") {
			log.Printf("[获取任务] 警告: ai_scheduled_tasks 表不存在，请运行 import_ai_tasks.bat 导入数据库表")
			respondJSON(w, http.StatusOK, []AIScheduledTask{})
			return
		}
		log.Printf("[获取任务] 统计记录数失败: %v", countErr)
	} else {
		log.Printf("[获取任务] 数据库中总共有 %d 条任务记录", totalCount)
	}

	query := `SELECT id, task_name, description, cron_expression, schedule_time, schedule_type,
		ai_role, ai_model, question, prompt_override, data_source, date_from, date_to,
		enable_wechat, wechat_room_id, is_active, last_run_at, last_run_status,
		last_run_result, next_run_at, total_runs, success_runs, failed_runs,
		created_at, updated_at
		FROM ai_scheduled_tasks ORDER BY created_at DESC`

	log.Printf("[获取任务] 执行查询SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		// 如果表不存在，返回空数组而不是报错
		if strings.Contains(err.Error(), "doesn't exist") || strings.Contains(err.Error(), "Table") {
			log.Printf("[获取任务] 警告: ai_scheduled_tasks 表不存在，请运行 import_ai_tasks.bat 导入数据库表")
			respondJSON(w, http.StatusOK, []AIScheduledTask{})
			return
		}
		log.Printf("[获取任务] 查询失败: %v", err)
		respondError(w, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	defer rows.Close()

	var tasks []AIScheduledTask = []AIScheduledTask{} // 初始化为空数组，不是nil
	taskCount := 0
	for rows.Next() {
		taskCount++
		var task AIScheduledTask
		var lastRunAt, nextRunAt, dateFrom, dateTo, cronExpr, lastRunResult, lastRunStatus, wechatRoomID sql.NullString
		var enableWechat, isActive sql.NullBool
		var totalRuns, successRuns, failedRuns sql.NullInt64

		var promptOverride sql.NullString
		err := rows.Scan(
			&task.ID, &task.TaskName, &task.Description, &cronExpr,
			&task.ScheduleTime, &task.ScheduleType, &task.AIRole, &task.AIModel,
			&task.Question, &promptOverride, &task.DataSource, &dateFrom, &dateTo,
			&enableWechat, &wechatRoomID, &isActive,
			&lastRunAt, &lastRunStatus, &lastRunResult, &nextRunAt,
			&totalRuns, &successRuns, &failedRuns,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			log.Printf("[获取任务] 扫描任务失败 (第%d条): %v", taskCount, err)
			continue
		}

		// 处理可能为NULL的字段
		if enableWechat.Valid {
			task.EnableWechat = enableWechat.Bool
		}
		if isActive.Valid {
			task.IsActive = isActive.Bool
		}
		if totalRuns.Valid {
			task.TotalRuns = int(totalRuns.Int64)
		}
		if successRuns.Valid {
			task.SuccessRuns = int(successRuns.Int64)
		}
		if failedRuns.Valid {
			task.FailedRuns = int(failedRuns.Int64)
		}

		if cronExpr.Valid {
			task.CronExpression = cronExpr.String
		}
		if lastRunAt.Valid {
			task.LastRunAt = lastRunAt.String
		}
		if lastRunStatus.Valid {
			task.LastRunStatus = lastRunStatus.String
		}
		if lastRunResult.Valid {
			task.LastRunResult = lastRunResult.String
		}
		if nextRunAt.Valid {
			task.NextRunAt = nextRunAt.String
		}
		if dateFrom.Valid {
			task.DateFrom = dateFrom.String
		}
		if dateTo.Valid {
			task.DateTo = dateTo.String
		}
		if wechatRoomID.Valid {
			task.WechatRoomID = wechatRoomID.String
		}
		if promptOverride.Valid {
			task.PromptOverride = promptOverride.String
		}

		tasks = append(tasks, task)
	}

	log.Printf("[获取任务] 查询完成，找到 %d 个任务", len(tasks))
	if len(tasks) > 0 {
		for i, t := range tasks {
			log.Printf("[获取任务] 任务 %d: ID=%d, Name=%s, Active=%v", i+1, t.ID, t.TaskName, t.IsActive)
		}
	} else {
		log.Printf("[获取任务] 任务列表为空，返回空数组")
	}

	// 确保返回的是数组而不是null
	if tasks == nil {
		tasks = []AIScheduledTask{}
	}

	log.Printf("[获取任务] 准备返回响应，任务数量: %d", len(tasks))
	respondJSON(w, http.StatusOK, tasks)
}

// getAITask 获取单个定时任务
func getAITask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	query := `SELECT id, task_name, description, cron_expression, schedule_time, schedule_type,
		ai_role, ai_model, question, prompt_override, data_source, date_from, date_to,
		enable_wechat, wechat_room_id, is_active, last_run_at, last_run_status,
		last_run_result, next_run_at, total_runs, success_runs, failed_runs,
		created_at, updated_at
		FROM ai_scheduled_tasks WHERE id = ?`

	var task AIScheduledTask
	var lastRunAt, nextRunAt, dateFrom, dateTo, cronExpr, lastRunResult, promptOverride sql.NullString
	err := db.QueryRow(query, id).Scan(
		&task.ID, &task.TaskName, &task.Description, &cronExpr,
		&task.ScheduleTime, &task.ScheduleType, &task.AIRole, &task.AIModel,
		&task.Question, &promptOverride, &task.DataSource, &dateFrom, &dateTo,
		&task.EnableWechat, &task.WechatRoomID, &task.IsActive,
		&lastRunAt, &task.LastRunStatus, &lastRunResult, &nextRunAt,
		&task.TotalRuns, &task.SuccessRuns, &task.FailedRuns,
		&task.CreatedAt, &task.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "任务不存在")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}

	if cronExpr.Valid {
		task.CronExpression = cronExpr.String
	}
	if lastRunAt.Valid {
		task.LastRunAt = lastRunAt.String
	}
	if lastRunResult.Valid {
		task.LastRunResult = lastRunResult.String
	}
	if nextRunAt.Valid {
		task.NextRunAt = nextRunAt.String
	}
	if dateFrom.Valid {
		task.DateFrom = dateFrom.String
	}
	if dateTo.Valid {
		task.DateTo = dateTo.String
	}
	if promptOverride.Valid {
		task.PromptOverride = promptOverride.String
	}

	respondJSON(w, http.StatusOK, task)
}

// createAITask 创建定时任务
func createAITask(w http.ResponseWriter, r *http.Request) {
	// 先读取请求体用于调试
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	log.Printf("[创建任务] 接收到原始请求体: %s", string(bodyBytes))

	var task AIScheduledTask
	if err := json.NewDecoder(bytes.NewBuffer(bodyBytes)).Decode(&task); err != nil {
		log.Printf("[创建任务] JSON解析失败: %v", err)
		respondError(w, http.StatusBadRequest, "请求数据格式错误: "+err.Error())
		return
	}

	log.Printf("[创建任务] 解析后的任务数据: TaskName=%s, ScheduleType=%s, ScheduleTime=%s, AIRole=%s, AIModel=%s, Question=%s, EnableWechat=%v, IsActive=%v",
		task.TaskName, task.ScheduleType, task.ScheduleTime, task.AIRole, task.AIModel, task.Question, task.EnableWechat, task.IsActive)

	// 验证必填字段
	if task.TaskName == "" {
		respondError(w, http.StatusBadRequest, "任务名称不能为空")
		return
	}
	if task.Question == "" {
		respondError(w, http.StatusBadRequest, "提问问题不能为空")
		return
	}

	// 计算下次执行时间
	nextRun := calculateNextRun(task.ScheduleType, task.ScheduleTime)
	log.Printf("[创建任务] 计算下次执行时间: %s", nextRun)

	// 如果没有提供 cron_expression，根据 schedule_type 和 schedule_time 生成
	if task.CronExpression == "" {
		task.CronExpression = generateCronExpression(task.ScheduleType, task.ScheduleTime)
		log.Printf("[创建任务] 生成 cron_expression: %s", task.CronExpression)
	}

	query := `INSERT INTO ai_scheduled_tasks (
		task_name, description, cron_expression, schedule_time, schedule_type,
		ai_role, ai_model, question, prompt_override, data_source, date_from, date_to,
		enable_wechat, wechat_room_id, is_active, next_run_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	log.Printf("[创建任务] 准备插入数据库，SQL参数: TaskName=%s, Description=%s, CronExpression=%s, ScheduleTime=%s, ScheduleType=%s, AIRole=%s, AIModel=%s, Question=%s, DataSource=%s, DateFrom=%s, DateTo=%s, EnableWechat=%v, WechatRoomID=%s, IsActive=%v, NextRun=%s",
		task.TaskName, task.Description, task.CronExpression, task.ScheduleTime,
		task.ScheduleType, task.AIRole, task.AIModel, task.Question,
		task.DataSource, task.DateFrom, task.DateTo,
		task.EnableWechat, task.WechatRoomID, task.IsActive, nextRun)

	result, err := db.Exec(query,
		task.TaskName, task.Description, task.CronExpression, task.ScheduleTime,
		task.ScheduleType, task.AIRole, task.AIModel, task.Question, task.PromptOverride,
		task.DataSource, nullString(task.DateFrom), nullString(task.DateTo),
		task.EnableWechat, task.WechatRoomID, task.IsActive, nextRun,
	)

	if err != nil {
		log.Printf("[创建任务] 数据库插入失败: %v", err)
		respondError(w, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("[创建任务] 获取插入ID失败: %v", err)
	} else {
		task.ID = int(id)
		log.Printf("[创建任务] 任务创建成功，ID: %d", id)

		// 立即验证数据是否真的插入成功
		var count int
		verifyErr := db.QueryRow("SELECT COUNT(*) FROM ai_scheduled_tasks WHERE id = ?", id).Scan(&count)
		if verifyErr != nil {
			log.Printf("[创建任务] 验证插入失败: %v", verifyErr)
		} else {
			log.Printf("[创建任务] 验证插入成功，数据库中任务数量: %d", count)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "任务创建成功",
		"task_id": id,
	})
}

// updateAITask 更新定时任务
func updateAITask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var task AIScheduledTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		respondError(w, http.StatusBadRequest, "请求数据格式错误")
		return
	}

	// 计算下次执行时间
	nextRun := calculateNextRun(task.ScheduleType, task.ScheduleTime)

	query := `UPDATE ai_scheduled_tasks SET
		task_name=?, description=?, cron_expression=?, schedule_time=?, schedule_type=?,
		ai_role=?, ai_model=?, question=?, prompt_override=?, data_source=?, date_from=?, date_to=?,
		enable_wechat=?, wechat_room_id=?, is_active=?, next_run_at=?
		WHERE id=?`

	_, err := db.Exec(query,
		task.TaskName, task.Description, task.CronExpression, task.ScheduleTime,
		task.ScheduleType, task.AIRole, task.AIModel, task.Question, task.PromptOverride,
		task.DataSource, nullString(task.DateFrom), nullString(task.DateTo),
		task.EnableWechat, task.WechatRoomID, task.IsActive, nextRun, id,
	)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "任务更新成功",
	})
}

// deleteAITask 删除定时任务
func deleteAITask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	_, err := db.Exec("DELETE FROM ai_scheduled_tasks WHERE id=?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "任务删除成功",
	})
}

// toggleAITask 启用/禁用任务
func toggleAITask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求数据格式错误")
		return
	}

	_, err := db.Exec("UPDATE ai_scheduled_tasks SET is_active=? WHERE id=?", req.IsActive, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	status := "禁用"
	if req.IsActive {
		status = "启用"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "任务已" + status,
	})
}

// executeAITask 立即执行任务
func executeAITask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// 查询任务配置
	var task AIScheduledTask
	query := `SELECT id, task_name, ai_role, ai_model, question, prompt_override, data_source, 
		date_from, date_to, enable_wechat, wechat_room_id
		FROM ai_scheduled_tasks WHERE id=?`

	var dateFrom, dateTo, promptOverride sql.NullString
	err := db.QueryRow(query, id).Scan(
		&task.ID, &task.TaskName, &task.AIRole, &task.AIModel,
		&task.Question, &promptOverride, &task.DataSource, &dateFrom, &dateTo,
		&task.EnableWechat, &task.WechatRoomID,
	)

	if err != nil {
		respondError(w, http.StatusNotFound, "任务不存在")
		return
	}

	if dateFrom.Valid {
		task.DateFrom = dateFrom.String
	}
	if dateTo.Valid {
		task.DateTo = dateTo.String
	}
	if promptOverride.Valid {
		task.PromptOverride = promptOverride.String
	}

	// 异步执行任务
	go runScheduledTask(task)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "任务已开始执行",
	})
}

// getAITaskHistory 获取任务执行历史
func getAITaskHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	query := `SELECT id, task_id, started_at, finished_at, status, ai_role, ai_model,
		question, data_count, ai_response, error_message, wechat_sent, wechat_result,
		created_at FROM ai_task_history WHERE task_id=? ORDER BY started_at DESC LIMIT 50`

	rows, err := db.Query(query, taskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	defer rows.Close()

	var history []AITaskHistory
	for rows.Next() {
		var h AITaskHistory
		var finishedAt sql.NullTime
		var errorMsg, wechatResult sql.NullString

		err := rows.Scan(
			&h.ID, &h.TaskID, &h.StartedAt, &finishedAt, &h.Status,
			&h.AIRole, &h.AIModel, &h.Question, &h.DataCount,
			&h.AIResponse, &errorMsg, &h.WechatSent, &wechatResult, &h.CreatedAt,
		)
		if err != nil {
			continue
		}

		if finishedAt.Valid {
			h.FinishedAt = finishedAt.Time
		}
		if errorMsg.Valid {
			h.ErrorMsg = errorMsg.String
		}
		if wechatResult.Valid {
			h.WechatResult = wechatResult.String
		}

		history = append(history, h)
	}

	respondJSON(w, http.StatusOK, history)
}

// ========== 任务执行逻辑 ==========

// runScheduledTask 执行定时任务
func runScheduledTask(task AIScheduledTask) {
	log.Printf("[定时任务] 开始执行任务: %s (ID:%d)", task.TaskName, task.ID)

	startTime := time.Now()
	historyID := createTaskHistory(task.ID, task.AIRole, task.AIModel, task.Question)

	// 更新任务状态为运行中
	db.Exec("UPDATE ai_task_history SET status='running' WHERE id=?", historyID)
	db.Exec("UPDATE ai_scheduled_tasks SET last_run_at=?, last_run_status='running' WHERE id=?",
		startTime, task.ID)

	// 1. 获取数据
	biddings, err := fetchBiddingsForTask(task)
	if err != nil {
		log.Printf("[定时任务] 获取数据失败: %v", err)
		updateTaskHistoryError(historyID, task.ID, err.Error())
		return
	}

	if len(biddings) == 0 {
		log.Printf("[定时任务] 没有找到符合条件的招标数据")
		updateTaskHistorySuccess(historyID, task.ID, 0, "没有找到符合条件的招标数据", "")
		return
	}

	log.Printf("[定时任务] 获取到 %d 条招标数据", len(biddings))

	// 2. 调用AI分析
	aiResponse, err := callAIForScheduledTask(task, biddings)
	if err != nil {
		log.Printf("[定时任务] AI分析失败: %v", err)
		updateTaskHistoryError(historyID, task.ID, err.Error())
		return
	}

	log.Printf("[定时任务] AI分析完成，响应长度: %d", len(aiResponse))

	// 3. 发送微信通知
	var wechatResult string
	if task.EnableWechat {
		result, err := postWechatMessage(task.WechatRoomID, aiResponse)
		if err != nil {
			log.Printf("[定时任务] 微信发送失败: %v", err)
			wechatResult = fmt.Sprintf("发送失败: %v", err)
		} else {
			log.Printf("[定时任务] 微信发送成功")
			wechatResult = fmt.Sprintf("发送成功: %+v", result)
		}
	}

	// 4. 更新执行结果
	updateTaskHistorySuccess(historyID, task.ID, len(biddings), aiResponse, wechatResult)

	log.Printf("[定时任务] 任务执行完成，耗时: %v", time.Since(startTime))
}

// fetchBiddingsForTask 根据任务配置获取招标数据
func fetchBiddingsForTask(task AIScheduledTask) ([]map[string]interface{}, error) {
	var query string
	var args []interface{}

	switch task.DataSource {
	case "today":
		// 使用与 getTodayBiddings 相同的逻辑：按本地时区的当天0点起算（UTC+8），再转换为UTC时间戳存储格式
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayStartStr := todayStart.UTC().Format(time.RFC3339)

		query = `SELECT id, serial, title, keywords, budget, 
			area, city, buyer, industry, publish_time, bid_end_time, site, created_at
			FROM bids 
			WHERE created_at >= ? AND (status = 0 OR status IS NULL)
			ORDER BY created_at DESC`
		args = append(args, todayStartStr)

	case "yesterday":
		query = `SELECT id, serial, title, keywords, budget,
			area, city, buyer, industry, publish_time, bid_end_time, site, created_at
			FROM bids WHERE DATE(created_at) = DATE_SUB(CURDATE(), INTERVAL 1 DAY) 
			ORDER BY created_at DESC`

	case "date_range":
		if task.DateFrom == "" || task.DateTo == "" {
			return nil, fmt.Errorf("日期范围模式需要指定起止日期")
		}
		query = `SELECT id, serial, title, keywords, budget,
			area, city, buyer, industry, publish_time, bid_end_time, site, created_at
			FROM bids WHERE DATE(created_at) BETWEEN ? AND ? ORDER BY created_at DESC`
		args = append(args, task.DateFrom, task.DateTo)

	default:
		return nil, fmt.Errorf("不支持的数据源类型: %s", task.DataSource)
	}

	log.Printf("[定时任务] 执行SQL查询: %s, args: %v", query, args)
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("[定时任务] SQL查询失败: %v", err)
		return nil, err
	}
	defer rows.Close()

	var biddings []map[string]interface{}
	rowCount := 0
	for rows.Next() {
		rowCount++
		var (
			id                      string
			publishTime, bidEndTime int64
			serial, title           string
			keywords, area, city    sql.NullString
			buyer, industry, site   sql.NullString
			budget                  float64
			createdAt               string
		)

		err := rows.Scan(&id, &serial, &title, &keywords, &budget,
			&area, &city, &buyer, &industry, &publishTime, &bidEndTime, &site, &createdAt)
		if err != nil {
			log.Printf("[定时任务] 扫描行数据失败 (行%d): %v", rowCount, err)
			continue
		}

		bid := map[string]interface{}{
			"id":           id,
			"serial":       serial,
			"title":        title,
			"budget":       budget,
			"publish_time": publishTime,
			"bid_end_time": bidEndTime,
			"created_at":   createdAt,
		}

		if keywords.Valid {
			bid["keywords"] = keywords.String
		}
		if area.Valid {
			bid["area"] = area.String
		}
		if city.Valid {
			bid["city"] = city.String
		}
		if buyer.Valid {
			bid["buyer"] = buyer.String
		}
		if industry.Valid {
			bid["industry"] = industry.String
		}
		if site.Valid {
			bid["site"] = site.String
		}

		biddings = append(biddings, bid)
	}

	log.Printf("[定时任务] SQL查询完成，扫描了 %d 行，返回 %d 条有效数据", rowCount, len(biddings))
	return biddings, nil
}

// callAIForScheduledTask 为定时任务调用AI
func callAIForScheduledTask(task AIScheduledTask, biddings []map[string]interface{}) (string, error) {
	// 构建系统提示词
	systemPrompt := getSystemPromptForRole(task.AIRole)
	if strings.TrimSpace(task.PromptOverride) != "" {
		systemPrompt = strings.TrimSpace(task.PromptOverride)
	}

	// 构建用户提示词
	userPrompt := task.Question + "\n\n招标数据：\n"
	for i, bid := range biddings {
		userPrompt += fmt.Sprintf("\n%d. 项目名称: %v", i+1, bid["title"])
		if keywords, ok := bid["keywords"]; ok && keywords != "" {
			userPrompt += fmt.Sprintf("\n   关键词: %v", keywords)
		}
		if budget, ok := bid["budget"].(float64); ok && budget > 0 {
			userPrompt += fmt.Sprintf("\n   预算: %.2f万元", budget/10000)
		}
		if area, ok := bid["area"]; ok {
			userPrompt += fmt.Sprintf("\n   地区: %v", area)
		}
		if buyer, ok := bid["buyer"]; ok {
			userPrompt += fmt.Sprintf("\n   采购单位: %v", buyer)
		}
		userPrompt += "\n"
	}

	response, _, err := callRelayStreamingAI(systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	return response, nil
}

// getSystemPromptForRole 根据角色获取系统提示词
func getSystemPromptForRole(roleKey string) string {
	var prompt string
	err := db.QueryRow("SELECT prompt FROM ai_roles WHERE role_key=?", roleKey).Scan(&prompt)
	if err != nil {
		// 使用默认提示词
		return "你是一个专业的招标分析助手，擅长分析招标信息并提供有价值的见解。"
	}
	return prompt
}

// ========== 任务历史管理 ==========

// createTaskHistory 创建任务执行历史记录
func createTaskHistory(taskID int, aiRole, aiModel, question string) int64 {
	result, err := db.Exec(
		`INSERT INTO ai_task_history (task_id, started_at, status, ai_role, ai_model, question)
		VALUES (?, NOW(), 'running', ?, ?, ?)`,
		taskID, aiRole, aiModel, question,
	)
	if err != nil {
		log.Printf("创建任务历史失败: %v", err)
		return 0
	}
	id, _ := result.LastInsertId()
	return id
}

// updateTaskHistorySuccess 更新任务历史为成功
func updateTaskHistorySuccess(historyID int64, taskID int, dataCount int, aiResponse, wechatResult string) {
	wechatSent := wechatResult != ""

	db.Exec(`UPDATE ai_task_history SET 
		finished_at=NOW(), status='success', data_count=?, ai_response=?,
		wechat_sent=?, wechat_result=?
		WHERE id=?`,
		dataCount, aiResponse, wechatSent, nullString(wechatResult), historyID)

	db.Exec(`UPDATE ai_scheduled_tasks SET 
		last_run_at=NOW(), last_run_status='success', 
		last_run_result=?, total_runs=total_runs+1, success_runs=success_runs+1,
		next_run_at=?
		WHERE id=?`,
		fmt.Sprintf("成功分析%d条数据", dataCount),
		calculateNextRunFromNow(taskID), taskID)
}

// updateTaskHistoryError 更新任务历史为失败
func updateTaskHistoryError(historyID int64, taskID int, errorMsg string) {
	db.Exec(`UPDATE ai_task_history SET 
		finished_at=NOW(), status='failed', error_message=?
		WHERE id=?`, errorMsg, historyID)

	db.Exec(`UPDATE ai_scheduled_tasks SET 
		last_run_at=NOW(), last_run_status='failed',
		last_run_result=?, total_runs=total_runs+1, failed_runs=failed_runs+1,
		next_run_at=?
		WHERE id=?`,
		errorMsg, calculateNextRunFromNow(taskID), taskID)
}

// ========== 任务调度辅助函数 ==========

// checkAndExecuteAITasks 检查并执行到期的AI定时任务
// 每分钟执行一次，查询所有启用的定时任务，判断是否到达执行时间
func checkAndExecuteAITasks() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[AI定时任务] 检查任务时发生panic: %v", r)
		}
	}()

	// 查询所有启用的定时任务
	query := `SELECT id, task_name, schedule_type, schedule_time, ai_role, ai_model, 
		question, data_source, date_from, date_to, enable_wechat, wechat_room_id,
		last_run_at
		FROM ai_scheduled_tasks 
		WHERE is_active = 1
		ORDER BY id`

	rows, err := db.Query(query)
	if err != nil {
		if !strings.Contains(err.Error(), "doesn't exist") {
			log.Printf("[AI定时任务] 查询任务失败: %v", err)
		}
		return
	}
	defer rows.Close()

	var tasks []AIScheduledTask
	for rows.Next() {
		var task AIScheduledTask
		var dateFrom, dateTo, lastRunAt sql.NullString

		err := rows.Scan(
			&task.ID, &task.TaskName, &task.ScheduleType, &task.ScheduleTime,
			&task.AIRole, &task.AIModel, &task.Question, &task.DataSource,
			&dateFrom, &dateTo, &task.EnableWechat, &task.WechatRoomID,
			&lastRunAt,
		)
		if err != nil {
			log.Printf("[AI定时任务] 扫描任务数据失败: %v", err)
			continue
		}

		if dateFrom.Valid {
			task.DateFrom = dateFrom.String
		}
		if dateTo.Valid {
			task.DateTo = dateTo.String
		}
		if lastRunAt.Valid {
			task.LastRunAt = lastRunAt.String
		}

		tasks = append(tasks, task)
	}

	if len(tasks) == 0 {
		return
	}

	log.Printf("[AI定时任务] 找到 %d 个启用的定时任务，开始检查执行时间...", len(tasks))

	// 检查每个任务是否需要执行
	for _, task := range tasks {
		if shouldExecuteTask(task) {
			log.Printf("[AI定时任务] 任务 '%s' (ID:%d) 已到执行时间，开始执行...", task.TaskName, task.ID)
			go runScheduledTask(task)
		}
	}
}

// shouldExecuteTask 判断任务是否应该执行
// 根据任务的调度类型（daily/weekly/once）和执行时间判断
// 避免在同一分钟内重复执行
func shouldExecuteTask(task AIScheduledTask) bool {
	now := time.Now()
	currentTime := now.Format("15:04")

	// 解析任务的执行时间
	if task.ScheduleTime == "" {
		return false
	}

	// 检查时间是否匹配（精确到分钟）
	if task.ScheduleTime != currentTime {
		return false
	}

	// 检查上次执行时间，避免在同一分钟内重复执行
	if task.LastRunAt != "" {
		lastRun, err := time.Parse("2006-01-02 15:04:05", task.LastRunAt)
		if err == nil {
			// 如果上次执行在1分钟以内，跳过
			if time.Since(lastRun) < 1*time.Minute {
				return false
			}
		}
	}

	// 根据任务类型判断
	switch task.ScheduleType {
	case "daily":
		// 每天执行
		return true

	case "weekly":
		// 每周执行（这里简化处理，可以后续扩展为指定星期几）
		// 例如：只在周一执行
		if now.Weekday() == time.Monday {
			return true
		}
		return false

	case "once":
		// 仅执行一次
		if task.LastRunAt == "" {
			return true
		}
		return false

	default:
		return false
	}
}

// generateCronExpression 根据 schedule_type 和 schedule_time 生成 cron 表达式
func generateCronExpression(scheduleType, scheduleTime string) string {
	switch scheduleType {
	case "daily":
		// 解析时间 "HH:MM:SS" 格式
		parts := strings.Split(scheduleTime, ":")
		if len(parts) >= 2 {
			hour := parts[0]
			minute := parts[1]
			// 每天执行: 分钟 小时 * * *
			return fmt.Sprintf("%s %s * * *", minute, hour)
		}
		return "0 9 * * *" // 默认每天9点
	case "weekly":
		parts := strings.Split(scheduleTime, ":")
		if len(parts) >= 2 {
			hour := parts[0]
			minute := parts[1]
			// 每周一执行: 分钟 小时 * * 1
			return fmt.Sprintf("%s %s * * 1", minute, hour)
		}
		return "0 9 * * 1" // 默认每周一9点
	case "once":
		return "" // 一次性任务不需要 cron
	default:
		return "0 9 * * *" // 默认每天9点
	}
}

// calculateNextRun 计算下次执行时间
func calculateNextRun(scheduleType, scheduleTime string) string {
	now := time.Now()

	switch scheduleType {
	case "daily":
		// 解析时间
		t, err := time.Parse("15:04:05", scheduleTime)
		if err != nil {
			return now.Add(24 * time.Hour).Format("2006-01-02 15:04:05")
		}

		nextRun := time.Date(now.Year(), now.Month(), now.Day(),
			t.Hour(), t.Minute(), t.Second(), 0, now.Location())

		// 如果今天的时间已过，设置为明天
		if nextRun.Before(now) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		return nextRun.Format("2006-01-02 15:04:05")

	case "weekly":
		// 每周执行，暂时设为7天后
		return now.Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05")

	case "once":
		// 一次性任务，不设下次执行
		return ""

	default:
		return now.Add(24 * time.Hour).Format("2006-01-02 15:04:05")
	}
}

// calculateNextRunFromNow 从当前时间计算下次执行时间
func calculateNextRunFromNow(taskID int) string {
	var scheduleType, scheduleTime string
	err := db.QueryRow("SELECT schedule_type, schedule_time FROM ai_scheduled_tasks WHERE id=?", taskID).
		Scan(&scheduleType, &scheduleTime)
	if err != nil {
		return time.Now().Add(24 * time.Hour).Format("2006-01-02 15:04:05")
	}
	return calculateNextRun(scheduleType, scheduleTime)
}
