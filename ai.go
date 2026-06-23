// AI分析核心功能模块
// 包含AI分析、AI调用、批量分析等核心功能
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ========== HTTP Handlers ==========

// getBidInsight 获取项目的小招 AI 情报速递（缓存优先，异步生成）
func getBidInsight(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 id 参数"})
		return
	}

	// 1. 检查缓存
	var insight sql.NullString
	err := db.QueryRowContext(r.Context(), "SELECT mascot_insight FROM bids WHERE id = ? LIMIT 1", id).Scan(&insight)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "项目不存在"})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if insight.Valid && strings.TrimSpace(insight.String) != "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"error_code": 0,
			"data":       map[string]string{"insight": insight.String, "cached": "true"},
		})
		return
	}

	// 2. 无缓存 → 立即返回占位，后台异步生成
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       map[string]string{"insight": "小招正在分析这个项目，稍后再来看看吧 ✦", "cached": "false"},
	})

	go func(bidID string) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[情报速递] 生成异常: %v", rec)
			}
		}()

		record, err := fetchBidRecordByID(bidID)
		if err != nil {
			log.Printf("[情报速递] 获取项目 %s 失败: %v", bidID, err)
			return
		}

		title := nullStringValue(record.Title)
		buyer := nullStringValue(record.Buyer)
		area := nullStringValue(record.Area)
		budget := 0.0
		if record.Budget.Valid {
			budget = record.Budget.Float64
		}

		prompt := fmt.Sprintf(
			"你是一个可爱的招标助理「小招AI」。请用一句话（不超过 40 个字）对下面这个招标项目做一个精炼的情报速递，语气要既专业又略带二次元萌感，可以带一个 emoji。\n\n项目名称：%s\n采购方：%s\n地区：%s\n预算：%.0f 元",
			title, buyer, area, budget,
		)

		systemPrompt := "你是小招AI，一个专业的招标情报助手。请只用一句话回复，不超过40字。"
		insightText, _, _, aiErr := runAIWorkflow(prompt, systemPrompt)
		if aiErr != nil {
			log.Printf("[情报速递] AI 生成失败: %v", aiErr)
			insightText = "小招暂时无法分析这个项目，请稍后重试 ✦"
		}

		insightText = strings.TrimSpace(insightText)
		if insightText == "" {
			insightText = "这个项目看起来很有趣，小招建议你深入研究一下 ✦"
		}

		if _, err := db.Exec("UPDATE bids SET mascot_insight = ? WHERE id = ?", insightText, bidID); err != nil {
			log.Printf("[情报速递] 存储失败: %v", err)
		} else {
			log.Printf("[情报速递] 项目 %s 情报已缓存", bidID)
		}
	}(id)
}

// chatWithAIHandler 与AI对话接口
// 处理用户与AI的对话请求，支持多轮对话
func chatWithAIHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages         []map[string]string      `json:"messages"`
		SelectedBiddings []map[string]interface{} `json:"selectedBiddings"`
		IncludeDetails   bool                     `json:"includeDetails"`
		AIRole           string                   `json:"aiRole"`
		Model            string                   `json:"model"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求: "+err.Error())
		return
	}

	// 默认角色
	if req.AIRole == "" {
		req.AIRole = "analyst"
	}

	// 从数据库读取角色提示词
	var systemPrompt string
	err := db.QueryRow("SELECT prompt FROM ai_roles WHERE role_key = ?", req.AIRole).Scan(&systemPrompt)
	if err != nil {
		if err == sql.ErrNoRows {
			// 如果角色不存在，使用默认提示词
			log.Printf("角色 %s 不存在，使用默认提示词", req.AIRole)
			systemPrompt = `你是一名专业的招标项目智能分析顾问，负责评估某物流运输企业是否应参与特定招标项目。

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

请用专业、简洁、有条理的方式回答问题。`
		} else {
			respondError(w, http.StatusInternalServerError, "查询角色失败: "+err.Error())
			return
		}
	}

	// 如果有选中的招标，将其添加到上下文中
	if len(req.SelectedBiddings) > 0 {
		var ctxBuilder strings.Builder
		ctxBuilder.WriteString("\n\n【当前选中的招标信息】\n")
		for i, bid := range req.SelectedBiddings {
			projectName := ""
			keywords := ""
			area := ""
			city := ""

			if pn, ok := bid["project_name"].(string); ok {
				projectName = pn
			}
			if kw, ok := bid["keywords"]; ok {
				if kwStr, ok := kw.(string); ok {
					keywords = kwStr
				}
			}
			if a, ok := bid["area"].(string); ok {
				area = a
			}
			if c, ok := bid["city"].(string); ok {
				city = c
			}

			location := strings.TrimSpace(area + " " + city)
			if location == "" {
				location = "未知"
			}

			fmt.Fprintf(&ctxBuilder, "项目%d：\n", i+1)
			fmt.Fprintf(&ctxBuilder, "  - 项目名称：%s\n", projectName)
			fmt.Fprintf(&ctxBuilder, "  - 关键词：%s\n", keywords)
			fmt.Fprintf(&ctxBuilder, "  - 地区：%s\n", location)

			// 如果需要包含详细信息
			if req.IncludeDetails {
				if desc, ok := bid["description"].(string); ok && desc != "" {
					fmt.Fprintf(&ctxBuilder, "  - 项目描述：%s\n", desc)
				}
				if detail, ok := bid["detail"].(string); ok && detail != "" {
					// 限制详情长度，避免过长
					if len(detail) > 500 {
						detail = detail[:500] + "..."
					}
					fmt.Fprintf(&ctxBuilder, "  - 公告详情：%s\n", detail)
				}
			}

			ctxBuilder.WriteByte('\n')
		}
		systemPrompt += ctxBuilder.String()
	}

	// 构建消息列表
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	messages = append(messages, req.Messages...)

	chatPrompt := ""
	for _, msg := range messages {
		if msg["role"] == "user" && strings.TrimSpace(msg["content"]) != "" {
			if chatPrompt != "" {
				chatPrompt += "\n\n"
			}
			chatPrompt += msg["content"]
		}
	}
	if chatPrompt == "" {
		chatPrompt = "请继续。"
	}

	reply, _, _, err := runAIWorkflow(chatPrompt, systemPrompt)
	if err != nil {
		log.Printf("AI 调用失败: %v", err)
		respondError(w, http.StatusInternalServerError, "AI 分析失败: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"reply": reply,
		},
	})
}

// analyzeBidHandler 分析招标项目
// 对指定的招标项目进行AI分析，评估是否适合投标
func analyzeBidHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	data, err := decodeJSONBody(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}

	// 检查是否只提供了 id 或 serial（新的调用方式）
	id := pickString(data, "id")
	serial := pickString(data, "serial")

	if id == "" && serial == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 id 或 serial"})
		return
	}

	// 如果只提供了 id 或 serial，没有其他详细字段，转发到 analyzeByIDHandler
	title := pickString(data, "title")
	detail := pickString(data, "detail")
	purchasing := pickString(data, "purchasing")

	hasDetailFields := title != "" || detail != "" || purchasing != ""

	if !hasDetailFields {
		// 转发到 analyzeByIDHandler（异步分析）
		var req struct {
			ID     string `json:"id"`
			Serial string `json:"serial"`
		}
		req.ID = id
		req.Serial = serial

		// 重新编码请求体
		newBody, _ := json.Marshal(req)
		r.Body = io.NopCloser(bytes.NewReader(newBody))

		// 调用 analyzeByIDHandler
		analyzeByIDHandler(w, r)
		return
	}

	// 以下是原有的详细分析逻辑（需要完整字段）
	keywords := pickString(data, "keywords")
	description := pickString(data, "description")
	shouldSkip, reasons := evaluateBidEligibility(title, stripHTMLCompact(detail), purchasing, keywords, description)
	if shouldSkip {
		analysis := buildEarlyExitAnalysis(reasons)
		analyzedAt, err := updateBidAIFields(id, analysis, "", "", "规则过滤")
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"error_code": 0,
			"data": map[string]interface{}{
				"id":               id,
				"analysis":         analysis,
				"ai_model":         "规则过滤",
				"analyzed_at":      analyzedAt,
				"thinking_process": "",
			},
		})
		return
	}
	defaultTemplate := `【招标项目信息】
- 标题：{{title}}
- 采购单位：{{buyer}}
- 项目名称：{{project_name}}
- 行业类型：{{industry}}
- 所在地区：{{area}} {{city}}
- 采购内容：{{purchasing}}
- 关键词：{{keywords}}
- 预算金额：{{budget}}
- 项目描述：{{description}}
- 详细内容：{{detail}}`
	projectTemplate := pickString(data, "customPrompt")
	if strings.TrimSpace(projectTemplate) == "" {
		projectTemplate = defaultTemplate
	}
	budgetValue := pickFloat(data, "budget")
	budgetText := "未知"
	if budgetValue > 0 {
		budgetText = fmt.Sprintf("%.2f万元", budgetValue/10000)
	}
	replacements := map[string]string{
		"title":        defaultString(title, "未知"),
		"buyer":        defaultString(pickString(data, "buyer"), "未知"),
		"project_name": defaultString(pickString(data, "project_name", "projectName"), "未知"),
		"industry":     defaultString(pickString(data, "industry"), "未知"),
		"area":         pickString(data, "area"),
		"city":         pickString(data, "city"),
		"purchasing":   defaultString(purchasing, "未知"),
		"keywords":     defaultString(keywords, "无"),
		"budget":       budgetText,
		"description":  defaultString(truncateRunes(stripHTMLCompact(description), 150), "无"),
		"detail":       defaultString(truncateRunes(stripHTMLCompact(firstNonEmpty(detail, description)), 100), "无"),
	}
	userPrompt := applyPromptTemplate(projectTemplate, replacements)
	systemPrompt := fetchSystemPrompt()
	preferredAI := strings.ToLower(pickString(data, "preferredAI"))
	if preferredAI == "" {
		preferredAI = "auto"
	}
	responseText, usedModel, thinking, aiErr := runAIWorkflow(userPrompt, systemPrompt)
	var analysis map[string]interface{}
	if aiErr != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "AI分析暂时不可用，未写入评分，请稍后重试",
		})
		return
	} else {
		parsed, parseErr := parseAnalysisJSON(responseText)
		if parseErr != nil {
			respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"error_code": 1,
				"error_msg":  "AI分析结果解析失败，未写入评分，请稍后重试",
			})
			return
		} else {
			analysis = parsed
		}
	}
	analyzedAt, err := updateBidAIFields(id, analysis, userPrompt, thinking, usedModel)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"id":               id,
			"analysis":         analysis,
			"ai_model":         usedModel,
			"analyzed_at":      analyzedAt,
			"prompt":           userPrompt,
			"thinking_process": thinking,
		},
	})
}

// analyzeByIDHandler 异步执行AI分析
func analyzeByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		ID     string `json:"id"`
		Serial string `json:"serial"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}

	var record *BidRecord
	var err error
	if req.ID != "" {
		record, err = fetchBidRecordByID(req.ID)
	} else if req.Serial != "" {
		record, err = fetchBidRecordBySerial(req.Serial)
	} else {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 id 或 serial 参数"})
		return
	}

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "未找到招标项目"})
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 立即返回响应，表示已接受分析请求
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "AI分析任务已启动，请稍后刷新查看结果",
		"data": map[string]interface{}{
			"id":     record.ID,
			"serial": nullStringValue(record.Serial),
			"status": "processing",
		},
	})

	// 异步执行AI分析
	go func(rec *BidRecord) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[AI分析] 项目 %s 分析时发生panic: %v", rec.ID, r)
			}
		}()

		log.Printf("[AI分析] 开始异步分析项目: %s", rec.ID)
		result, err := performBidAnalysis(rec, "", "auto", "")
		if err != nil {
			log.Printf("[AI分析] 项目 %s 分析失败: %v", rec.ID, err)
		} else {
			log.Printf("[AI分析] 项目 %s 分析完成，模型: %s", rec.ID, result.AIModel)

			// 发送微信通知
			go sendWechatAfterSingleAnalysis(rec, result)
		}
	}(record)
}

// reanalyzeBidHandler 重新分析招标项目
func reanalyzeBidHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		ID           string `json:"id"`
		Serial       string `json:"serial"`
		CustomPrompt string `json:"customPrompt"`
		PreferredAI  string `json:"preferredAI"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}

	var record *BidRecord
	var err error
	if req.ID != "" {
		record, err = fetchBidRecordByID(req.ID)
	} else if req.Serial != "" {
		record, err = fetchBidRecordBySerial(req.Serial)
	} else {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 id 或 serial 参数"})
		return
	}

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "未找到招标项目"})
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	preferred := strings.ToLower(strings.TrimSpace(req.PreferredAI))
	if preferred == "" {
		preferred = "auto"
	}

	log.Printf("[AI重新分析] 开始同步重新分析项目: %s", record.ID)
	result, analysisErr := performBidAnalysis(record, req.CustomPrompt, preferred, "")
	if analysisErr != nil {
		log.Printf("[AI重新分析] 项目 %s 分析失败: %v", record.ID, analysisErr)
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error_code": 1,
			"error_msg":  analysisErr.Error(),
		})
		return
	}

	log.Printf("[AI重新分析] 项目 %s 分析完成，模型: %s", record.ID, result.AIModel)
	go sendWechatAfterSingleAnalysis(record, result)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "重新分析完成",
		"data": map[string]interface{}{
			"id":               result.ID,
			"serial":           result.Serial,
			"analysis":         result.Analysis,
			"ai_model":         result.AIModel,
			"prompt":           result.Prompt,
			"raw_response":     result.RawResponse,
			"thinking_process": result.Thinking,
			"analyzed_at":      result.AnalyzedAt,
		},
	})
}

// batchAnalyzeHandler 批量分析招标项目
func batchAnalyzeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 ids 参数"})
		return
	}

	// 验证所有ID是否存在
	validIDs := make([]string, 0)
	for _, id := range req.IDs {
		record, err := fetchBidRecordByID(id)
		if err == nil && record != nil {
			validIDs = append(validIDs, id)
		}
	}

	if len(validIDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "没有找到有效的项目",
		})
		return
	}

	// 立即返回响应
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    fmt.Sprintf("批量分析任务已启动，共 %d 个项目", len(validIDs)),
		"data": map[string]interface{}{
			"total":  len(validIDs),
			"status": "processing",
			"ids":    validIDs,
		},
	})

	// 异步执行批量分析
	go func(ids []string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[批量AI分析] 发生panic: %v", r)
			}
		}()

		log.Printf("[批量AI分析] 开始异步分析 %d 个项目", len(ids))
		successCount := 0
		for i, id := range ids {
			record, err := fetchBidRecordByID(id)
			if err != nil {
				log.Printf("[批量AI分析] 项目 %s 查询失败: %v", id, err)
				continue
			}

			result, analysisErr := performBidAnalysis(record, "", "auto", "")
			if analysisErr != nil {
				log.Printf("[批量AI分析] 项目 %s 分析失败: %v", id, analysisErr)
			} else {
				successCount++
				log.Printf("[批量AI分析] 项目 %s 分析完成 (%d/%d)，模型: %s", id, i+1, len(ids), result.AIModel)
			}

			// 每个项目之间间隔1秒，避免过快请求
			if i < len(ids)-1 {
				time.Sleep(1 * time.Second)
			}
		}
		log.Printf("[批量AI分析] 批量分析完成，成功: %d/%d", successCount, len(ids))
	}(validIDs)
}

// kimiAIHandler Kimi AI格式化处理
func kimiAIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		Serial string `json:"serial"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Serial) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 serial 参数"})
		return
	}
	record, err := fetchBidRecordBySerial(req.Serial)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "未找到招标项目"})
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rawDetail := stripHTMLPreserveBreaks(nullStringValue(record.Detail))
	if rawDetail == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "项目暂无可排版的详情内容"})
		return
	}

	formatted, model, err := callKimiFormatter(rawDetail)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": err.Error()})
		return
	}

	_, updateErr := db.Exec("UPDATE bids SET detail = ? WHERE serial = ?", formatted, req.Serial)
	if updateErr != nil {
		log.Printf("[Kimi格式化] 更新数据库失败: %v", updateErr)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"formatted": formatted,
			"model":     model,
			"serial":    req.Serial,
		},
	})
}

// ========== AI工作流 ==========

// runAIWorkflow AI工作流，根据 AI_PROVIDER 配置选择 relay 或 ctyun 直连
func runAIWorkflow(userPrompt, systemPrompt string) (string, string, string, error) {
	provider := getConfigValue("AI_PROVIDER", "ctyun")

	if provider == "ctyun" {
		log.Printf("[AI工作流] 使用 CTYun 直连")
		return callDirectCTYunAI(systemPrompt, userPrompt)
	}

	log.Printf("[AI工作流] 使用 Relay 中转")
	response, thinking, err := callRelayStreamingAI(systemPrompt, userPrompt)
	if err != nil {
		log.Printf("[AI工作流] Relay AI调用失败: %v", err)
		return "", "", "", err
	}
	return response, fmt.Sprintf("Relay AI (%s)", getConfigValue("RELAY_AI_MODEL", defaultRelayAIModel)), thinking, nil
}

// ========== 数据获取和提示词构建 ==========

// fetchBidRecordByField 根据字段查询招标记录
func fetchBidRecordByField(field, value string) (*BidRecord, error) {
	if value == "" {
		return nil, errors.New("missing identifier")
	}
	query := fmt.Sprintf(`
		SELECT id, serial, title, buyer, project_name, project_code, industry, area, city, district,
			purchasing, keywords, budget, description, detail, pdf_url, source,
			public_type, top_type, sub_type, deliver_area, deliver_city, deliver_detail,
			china_tender_project_name, china_bid_section_name, china_notice_name, china_classify_name
		FROM bids WHERE %s = ? LIMIT 1
	`, field)
	var record BidRecord
	err := db.QueryRow(query, value).Scan(
		&record.ID,
		&record.Serial,
		&record.Title,
		&record.Buyer,
		&record.ProjectName,
		&record.ProjectCode,
		&record.Industry,
		&record.Area,
		&record.City,
		&record.District,
		&record.Purchasing,
		&record.Keywords,
		&record.Budget,
		&record.Description,
		&record.Detail,
		&record.PDFURL,
		&record.Source,
		&record.PublicType,
		&record.TopType,
		&record.SubType,
		&record.DeliverArea,
		&record.DeliverCity,
		&record.DeliverDetail,
		&record.ChinaTenderProjectName,
		&record.ChinaBidSectionName,
		&record.ChinaNoticeName,
		&record.ChinaClassifyName,
	)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// fetchBidRecordByID 根据ID查询招标记录
func fetchBidRecordByID(id string) (*BidRecord, error) {
	return fetchBidRecordByField("id", id)
}

// fetchBidRecordBySerial 根据编号查询招标记录
func fetchBidRecordBySerial(serial string) (*BidRecord, error) {
	return fetchBidRecordByField("serial", serial)
}

// fetchSystemPrompt 获取系统提示词
func fetchSystemPrompt() string {
	var prompt sql.NullString
	err := db.QueryRow("SELECT value FROM configs WHERE `key` = 'ai_prompt' LIMIT 1").Scan(&prompt)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("warn: fetch system prompt failed: %v", err)
	}
	if prompt.Valid && strings.TrimSpace(prompt.String) != "" {
		return prompt.String
	}
	return "你是一名专业的招标项目智能分析顾问，负责评估物流运输企业是否应参与特定招标项目。请根据用户提供的项目信息，进行专业分析并以 JSON 格式返回结果。"
}

// applyPromptTemplate 应用提示词模板
func applyPromptTemplate(template string, replacements map[string]string) string {
	result := template
	for key, value := range replacements {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// buildBidPrompt 构建招标项目提示词
func buildBidPrompt(record *BidRecord, customPrompt string) string {
	basePrompt := buildFullBidPromptText(record)
	if strings.TrimSpace(customPrompt) != "" {
		return basePrompt + "\n\n【补充要求】\n" + strings.TrimSpace(customPrompt)
	}
	return basePrompt
}

// ========== 核心分析函数 ==========

// performBidAnalysis 执行招标项目分析
func performBidAnalysis(record *BidRecord, customPrompt, preferredAI, cfModel string) (*aiComputationResult, error) {
	log.Printf("[AI分析详细] 步骤1: 开始分析项目 %s", record.ID)

	systemPrompt := fetchSystemPrompt()
	log.Printf("[AI分析详细] 步骤2: 获取系统提示词成功，长度: %d", len(systemPrompt))

	// 构建用户提示词：统一使用全字段提示词
	var userPrompt string
	if customPrompt != "" {
		userPrompt = customPrompt
		log.Printf("[AI分析详细] 步骤3: 使用自定义提示词，长度: %d", len(userPrompt))
	} else {
		log.Printf("[AI分析详细] 步骤3: 使用全字段模板构建提示词")
		userPrompt = buildBidPrompt(record, "")
		log.Printf("[AI分析详细] 步骤3完成: 全字段模板构建完成，长度: %d", len(userPrompt))
	}

	log.Printf("[AI分析详细] 步骤4: 开始调用AI服务，preferredAI=%s, cfModel=%s", preferredAI, cfModel)
	responseText, usedModel, thinking, err := runBidAIWorkflow(record, userPrompt, systemPrompt)
	if err != nil {
		// 所有 AI 服务都不可用，返回错误而不更新数据库
		log.Printf("[AI分析详细] 步骤4失败: AI服务调用失败 - %v", err)
		return nil, fmt.Errorf("所有AI服务暂时不可用: %v", err)
	}
	log.Printf("[AI分析详细] 步骤4完成: AI响应成功，模型=%s, 响应长度=%d", usedModel, len(responseText))
	log.Printf("[AI分析详细] 步骤4 raw_response=[%s]", responseText[:min(len(responseText), 500)])

	log.Printf("[AI分析详细] 步骤5: 开始解析AI响应JSON")
	analysis, parseErr := parseAnalysisJSON(responseText)
	if parseErr != nil {
		log.Printf("[AI分析详细] 步骤5失败: JSON解析失败，不写入数据库 - %v", parseErr)
		return nil, fmt.Errorf("AI分析结果解析失败: %v", parseErr)
	}
	log.Printf("[AI分析详细] 步骤5完成: JSON解析成功，Score=%d", getScoreFromAnalysis(analysis))

	log.Printf("[AI分析详细] 步骤6: 开始更新数据库，项目ID=%s", record.ID)
	analyzedAt, err := updateBidAIFields(record.ID, analysis, userPrompt, thinking, usedModel)
	if err != nil {
		log.Printf("[AI分析详细] 步骤6失败: 数据库更新失败 - %v", err)
		return nil, err
	}
	log.Printf("[AI分析详细] 步骤6完成: 数据库更新成功，分析时间=%s", analyzedAt)

	log.Printf("[AI分析详细] 步骤7: 所有步骤完成，返回结果")
	return &aiComputationResult{
		ID:          record.ID,
		Serial:      nullStringValue(record.Serial),
		Analysis:    analysis,
		AIModel:     usedModel,
		Prompt:      userPrompt,
		RawResponse: responseText,
		Thinking:    thinking,
		AnalyzedAt:  analyzedAt,
	}, nil
}

func downloadFileForAI(fileURL string) (string, error) {
	if strings.TrimSpace(fileURL) == "" {
		return "", errors.New("empty file url")
	}

	req, err := http.NewRequest(http.MethodGet, fileURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("download file failed %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if err := os.MkdirAll("/tmp/ai_bid_files", 0o755); err != nil {
		return "", err
	}
	filename := filepath.Base(fileURL)
	if filename == "" || filename == "." || filename == "/" {
		filename = fmt.Sprintf("%s.pdf", recordSafeID(fileURL))
	}
	// 中转 AI 的 /api/files/upload 仅接受白名单后缀（pdf/doc/docx/xls/xlsx/ppt/pptx/txt/md...）。
	// 链接以 .html/.htm 结尾时改写成 .txt，避免被判"不支持的文件类型"。
	lowerName := strings.ToLower(filename)
	if strings.HasSuffix(lowerName, ".html") || strings.HasSuffix(lowerName, ".htm") {
		filename = strings.TrimSuffix(strings.TrimSuffix(filename, ".html"), ".htm") + ".txt"
	}
	localPath := filepath.Join("/tmp/ai_bid_files", filename)
	out, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}
	return localPath, nil
}

func recordSafeID(s string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "?", "_", "&", "_", "=", "_")
	return replacer.Replace(s)
}

func isLikelyHTMLSourceURL(fileURL string) bool {
	lowerURL := strings.ToLower(strings.TrimSpace(fileURL))
	if lowerURL == "" {
		return false
	}
	if idx := strings.Index(lowerURL, "?"); idx >= 0 {
		lowerURL = lowerURL[:idx]
	}
	return strings.HasSuffix(lowerURL, ".html") || strings.HasSuffix(lowerURL, ".htm")
}

func buildHTMLFileForAI(record *BidRecord) (string, error) {
	content := strings.TrimSpace(nullStringValue(record.Detail))
	if content == "" {
		content = strings.TrimSpace(nullStringValue(record.Description))
	}
	if content == "" {
		return "", errors.New("empty html content")
	}

	if !strings.Contains(strings.ToLower(content), "<html") {
		content = fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>%s</title></head><body>%s</body></html>`,
			defaultString(nullStringValue(record.Title), "bid-detail"), content)
	}

	if err := os.MkdirAll("/tmp/ai_bid_files", 0o755); err != nil {
		return "", err
	}

	fileToken := record.ID
	if fileToken == "" {
		fileToken = nullStringValue(record.Serial)
	}
	if fileToken == "" {
		fileToken = time.Now().Format("20060102150405")
	}

	// 注意：中转 AI 的 /api/files/upload 不接受 .html 后缀（返回"不支持的文件类型"）。
	// 用 .txt 保存 HTML 字符串内容，AI 仍能阅读其中的招标信息（标签会被当作文本流）。
	localPath := filepath.Join("/tmp/ai_bid_files", fmt.Sprintf("%s.txt", recordSafeID(fileToken)))
	if err := os.WriteFile(localPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return localPath, nil
}

func buildFullBidPromptText(record *BidRecord) string {
	budgetText := "未知"
	budget := nullFloatValue(record.Budget)
	if budget > 0 {
		budgetText = fmt.Sprintf("%.2f万元", budget/10000)
	}

	fields := []struct {
		label string
		value string
	}{
		{"数据来源", nullStringValue(record.Source)},
		{"标题", defaultString(nullStringValue(record.Title), "未知")},
		{"项目编号", nullStringValue(record.Serial)},
		{"项目名称", nullStringValue(record.ProjectName)},
		{"项目编码", nullStringValue(record.ProjectCode)},
		{"招标项目名称", nullStringValue(record.ChinaTenderProjectName)},
		{"投标标段名称", nullStringValue(record.ChinaBidSectionName)},
		{"公告名称", nullStringValue(record.ChinaNoticeName)},
		{"分类名称", nullStringValue(record.ChinaClassifyName)},
		{"采购单位", nullStringValue(record.Buyer)},
		{"行业类型", nullStringValue(record.Industry)},
		{"所在地区", strings.TrimSpace(strings.Join([]string{
			nullStringValue(record.Area),
			nullStringValue(record.City),
			nullStringValue(record.District),
		}, " "))},
		{"采购内容", nullStringValue(record.Purchasing)},
		{"公告类型", nullStringValue(record.PublicType)},
		{"一级分类", nullStringValue(record.TopType)},
		{"二级分类", nullStringValue(record.SubType)},
		{"交付区域", nullStringValue(record.DeliverArea)},
		{"交付城市", nullStringValue(record.DeliverCity)},
		{"交付详情", nullStringValue(record.DeliverDetail)},
		{"关键词", defaultString(nullStringValue(record.Keywords), "无")},
		{"预算金额", budgetText},
		{"项目描述", defaultString(strings.TrimSpace(nullStringValue(record.Description)), "无")},
		{"详细内容", defaultString(strings.TrimSpace(nullStringValue(record.Detail)), "无")},
	}

	var builder strings.Builder
	builder.WriteString("【招标项目信息】\n")
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("- %s：%s\n", field.label, field.value))
	}
	return strings.TrimSpace(builder.String())
}

func shouldUploadPromptAsText(prompt string) bool {
	return len([]rune(prompt)) > 1200
}

// modelSupportsFiles 判断当前选择的中转 AI 模型是否支持文件问答。
// 根据中转 AI 服务 /docs 接口（最新版）：
//   - 所有 CTYun TEXT_* 模型均支持文件问答，统一走 /api/files/upload；
//     联网搜索 + 深度思考由中转强制开启。
//   - lingxi 是独立链路，文件上传走 /api/lingxi/files/upload-share。
//   - 其它形态的模型 key 视为不支持，直接走纯文本流程。
func modelSupportsFiles() bool {
	fileModel := strings.TrimSpace(getRelayAIFileModel())
	lower := strings.ToLower(fileModel)
	switch lower {
	case "lingxi":
		return true
	case "deep", "default":
		// 旧别名 → DeepSeek-V4
		return true
	}
	if strings.HasPrefix(strings.ToUpper(fileModel), "TEXT_") {
		return true
	}
	if fileModel == "" {
		general := strings.TrimSpace(getConfigValue("RELAY_AI_MODEL", defaultRelayAIModel))
		if strings.HasPrefix(strings.ToUpper(general), "TEXT_") || general == "" {
			return true
		}
	}
	return false
}

func buildPromptTextFileForAI(record *BidRecord, prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("empty prompt text")
	}
	if err := os.MkdirAll("/tmp/ai_bid_files", 0o755); err != nil {
		return "", err
	}
	fileToken := record.ID
	if fileToken == "" {
		fileToken = nullStringValue(record.Serial)
	}
	if fileToken == "" {
		fileToken = time.Now().Format("20060102150405")
	}
	localPath := filepath.Join("/tmp/ai_bid_files", fmt.Sprintf("%s-prompt.md", recordSafeID(fileToken)))
	if err := os.WriteFile(localPath, []byte(prompt), 0o644); err != nil {
		return "", err
	}
	return localPath, nil
}

func runBidAIWorkflow(record *BidRecord, userPrompt, systemPrompt string) (string, string, string, error) {
	// 当前选用的中转 AI 模型不支持文件问答时，直接走纯文本流程，避免无谓的 /api/files/upload 失败回退
	supportsFiles := modelSupportsFiles()
	if !supportsFiles {
		log.Printf("[AI工作流] 当前文件模型 %q 不支持文件问答，直接走普通AI流程", getRelayAIFileModel())
		return runAIWorkflow(userPrompt, systemPrompt)
	}

	// 关键：保留原始 prompt。文件流程任何一步失败时，回退到普通 AI 流程要用回完整原文，
	// 否则 AI 拿到空泛指令 + 系统提示词的公司画像，会瞎编（如把"厄瓜多尔油管船"当成"云南烟草运输"）。
	originalUserPrompt := userPrompt

	provider := getConfigValue("AI_PROVIDER", "ctyun")

	// CTYun 直连：始终将提示词转为文件上传，短指令提问
	if provider == "ctyun" && ctyunClient != nil {
		localPaths := make([]string, 0)

		// 1. 构建完整提示词 = 数据库 ai_prompt + 招标数据
		dbPrompt := fetchSystemPrompt()
		fullPrompt := strings.ReplaceAll(dbPrompt, "{{bid_info}}", originalUserPrompt)
		if !strings.Contains(dbPrompt, "{{bid_info}}") {
			fullPrompt = dbPrompt + "\n\n【招标项目信息】\n" + originalUserPrompt
		}

		// 2. 将完整提示词写入文件
		if promptPath, err := buildPromptTextFileForAI(record, fullPrompt); err == nil {
			localPaths = append(localPaths, promptPath)
			log.Printf("[AI工作流] [CTYun] 提示词已转为文件(%d bytes): %s", len(fullPrompt), promptPath)
		}

		// 3. 有附件时处理（china 来源的 HTML 直接用数据库 detail 字段，不下载）
		pdfURL := strings.TrimSpace(nullStringValue(record.PDFURL))
		if pdfURL != "" {
			if strings.EqualFold(nullStringValue(record.Source), "china") && isLikelyHTMLSourceURL(pdfURL) {
				// china 来源：优先用数据库 detail 字段，为空则下载 HTML
				if localPath, err := buildHTMLFileForAI(record); err == nil {
					localPaths = append(localPaths, localPath)
					log.Printf("[AI工作流] [CTYun] 已从 detail 生成本地文件: %s", localPath)
				} else {
					log.Printf("[AI工作流] [CTYun] 数据库 detail 为空，尝试下载 HTML: %v", err)
					if localPath, err := downloadFileForAI(pdfURL); err == nil {
						localPaths = append(localPaths, localPath)
						log.Printf("[AI工作流] [CTYun] HTML 下载成功: %s", localPath)
					} else {
						log.Printf("[AI工作流] [CTYun] HTML 下载也失败，跳过附件: %v", err)
					}
				}
			} else {
				// 非 china 来源或 PDF 直链：下载文件
				log.Printf("[AI工作流] [CTYun] 下载附件: %s", pdfURL)
				if localPath, err := downloadFileForAI(pdfURL); err == nil {
					localPaths = append(localPaths, localPath)
				} else {
					log.Printf("[AI工作流] [CTYun] 附件下载失败，跳过: %v", err)
				}
			}
		}

		// 4. 上传所有文件到 CTYun
		var ctyunFiles []CTYunUploadedFile
		if len(localPaths) > 0 {
			if err := ctyunClient.EnsureSession(); err != nil {
				log.Printf("[AI工作流] [CTYun] 会话失效: %v", err)
			} else {
				xuid := "pubweb_" + ctyRandomHex(16)
				if files, err := ctyunClient.UploadFiles(localPaths, xuid); err == nil {
					ctyunFiles = files
					log.Printf("[AI工作流] [CTYun] 上传 %d 个文件成功", len(files))
				}
			}
		}

		model := getConfigValue("CTYUN_MODEL", "TEXT_DEEPSEEK_V4")
		log.Printf("[AI工作流] [CTYun] 分析，模型=%s，文件数=%d", model, len(ctyunFiles))
		// 发短指令，完整提示词已写入 .md 文件上传给 AI
		shortInstruction := `请根据已上传的 readme.md 文件回答问题。

【输出要求】
- 严格输出 JSON，字段：suitable, score, matchLevel, dimensionScores, reasons, advantages, risks, recommendation, priority
- dimensionScores 包含 geographicFit, industryRelevance, projectScaleFit 三个键（数字）
- reasons / advantages / risks 都是字符串数组
- 任何 score ≥ 80 的回答，必须在 reasons 中至少给出 3 条与原文【招标项目信息】里具体字段对应的证据
- 不要包含示例数据，不要任何 JSON 之外的自然语言或解释`
		responseText, thinkingText, chatErr := callDirectCTYunWithFiles(systemPrompt, shortInstruction, ctyunFiles, model)
		if chatErr == nil {
			return responseText, fmt.Sprintf("CTYun (%s)", model), thinkingText, nil
		}
		log.Printf("[AI工作流] [CTYun] 分析失败: %v，回退", chatErr)
		return runAIWorkflow(originalUserPrompt, systemPrompt)
	}

	extraFilePaths := make([]string, 0)
	if shouldUploadPromptAsText(userPrompt) {
		if promptPath, err := buildPromptTextFileForAI(record, userPrompt); err != nil {
			log.Printf("[AI工作流] 生成提问TXT失败，继续走纯文本提示: %v", err)
		} else {
			extraFilePaths = append(extraFilePaths, promptPath)
			log.Printf("[AI工作流] 提问内容较长，已转为TXT文件上传: %s", promptPath)
			userPrompt = "请结合已上传的项目资料文件，完成评分与分析，并严格按要求输出结果。"
		}
	}

	pdfURL := strings.TrimSpace(nullStringValue(record.PDFURL))
	if pdfURL != "" || len(extraFilePaths) > 0 {
		log.Printf("[AI工作流] 检测到源文件链接，优先走文件问答: %s", pdfURL)
		localPaths := make([]string, 0)
		localPaths = append(localPaths, extraFilePaths...)
		if pdfURL != "" {
			var (
				localPath string
				err       error
			)

			if strings.EqualFold(nullStringValue(record.Source), "china") && isLikelyHTMLSourceURL(pdfURL) {
				localPath, err = buildHTMLFileForAI(record)
				if err != nil {
					log.Printf("[AI工作流] 生成HTML文件失败，回退到直接下载链接: %v", err)
				} else {
					log.Printf("[AI工作流] 已为 china HTML 链接生成本地文件: %s", localPath)
				}
			}

			if localPath == "" {
				localPath, err = downloadFileForAI(pdfURL)
			}
			if err != nil {
				log.Printf("[AI工作流] 源文件获取失败，回退普通AI流程: %v", err)
				return runAIWorkflow(originalUserPrompt, systemPrompt)
			}
			localPaths = append(localPaths, localPath)
		}

		var files []map[string]interface{}
		if getRelayAIFileModel() == "lingxi" {
			var sharedURL string
			for index, path := range localPaths {
				uploadedFiles, uploadedSharedURL, uploadErr := uploadLingxiSharedFile(path)
				if uploadErr != nil {
					log.Printf("[AI工作流] 灵析共享上传失败，回退普通AI流程: %v", uploadErr)
					return runAIWorkflow(originalUserPrompt, systemPrompt)
				}
				files = append(files, uploadedFiles...)
				if index == len(localPaths)-1 && uploadedSharedURL != "" && pdfURL != "" {
					sharedURL = uploadedSharedURL
				}
			}
			if sharedURL != "" {
				if _, updateErr := db.Exec("UPDATE bids SET original_pdf_url = COALESCE(original_pdf_url, pdf_url), pdf_url = ? WHERE id = ?", sharedURL, record.ID); updateErr != nil {
					log.Printf("[AI工作流] 更新灵析共享链接失败: %v", updateErr)
				} else {
					log.Printf("[AI工作流] 已将项目 %s 的 pdf_url 更新为灵析共享链接", record.ID)
				}
			}
		} else {
			fileIDs := make([]string, 0)
			for _, path := range localPaths {
				uploadedFileIDs, uploadErr := uploadRelayAIFile(path)
				if uploadErr != nil {
					log.Printf("[AI工作流] 文件上传失败，回退普通AI流程: %v", uploadErr)
					return runAIWorkflow(originalUserPrompt, systemPrompt)
				}
				fileIDs = append(fileIDs, uploadedFileIDs...)
			}

			readyFiles, readyErr := waitRelayAIFileReady(fileIDs)
			if readyErr != nil {
				log.Printf("[AI工作流] PDF解析失败，回退普通AI流程: %v", readyErr)
				return runAIWorkflow(originalUserPrompt, systemPrompt)
			}
			files = readyFiles
		}

		response, thinking, err := callRelayStreamingAIWithFiles(systemPrompt, userPrompt, files)
		if err == nil {
			modelLabel := getRelayAIFileModel()
			if getRelayAIFileModel() == "lingxi" {
				modelLabel = "lingxi"
			}
			return response, fmt.Sprintf("Relay AI File (%s)", modelLabel), thinking, nil
		}
		log.Printf("[AI工作流] 文件问答失败，回退普通AI流程: %v", err)
	}

	return runAIWorkflow(originalUserPrompt, systemPrompt)
}

// ========== 响应解析和处理 ==========

// splitThinkingContent 分离思考过程和响应内容
func splitThinkingContent(response string) (string, string) {
	if response == "" {
		return "", ""
	}
	match := thinkTagPattern.FindStringSubmatch(response)
	if len(match) == 2 {
		idx := strings.Index(response, match[0])
		clean := response
		if idx >= 0 {
			clean = response[idx+len(match[0]):]
		}
		return strings.TrimSpace(clean), strings.TrimSpace(match[1])
	}
	return strings.TrimSpace(response), ""
}

// parseAnalysisJSON 解析AI响应JSON
func parseAnalysisJSON(responseText string) (map[string]interface{}, error) {
	// 直接用 extractJSONObject 从原始响应中提取 JSON，不依赖前缀/代码块处理
	log.Printf("[JSON解析] 原始响应前40字节 hex: %x", []byte(responseText[:min(len(responseText), 40)]))

	// 先尝试从原始文本提取 JSON 对象
	jsonPayload, err := extractJSONObject(responseText)
	if err != nil {
		// 如果失败，去掉可能的前缀（如 "json\n"）再试
		cleaned := strings.TrimSpace(responseText)
		if len(cleaned) > 4 && strings.ToLower(cleaned[:4]) == "json" {
			cleaned = strings.TrimSpace(cleaned[4:])
			jsonPayload, err = extractJSONObject(cleaned)
		}
	}
	if err != nil {
		log.Printf("[JSON解析] 所有提取方式失败: %v", err)
		return nil, err
	}

	decoder := json.NewDecoder(strings.NewReader(jsonPayload))
	decoder.UseNumber()
	var analysis map[string]interface{}
	if err := decoder.Decode(&analysis); err != nil {
		return nil, err
	}
	return analysis, nil
}

// extractJSONObject 从文本中提取JSON对象

func extractJSONObject(text string) (string, error) {
	start := -1
	depth := 0
	inString := false
	escape := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\\' && !escape {
			escape = true
			continue
		}
		if ch == '"' && !escape {
			inString = !inString
		}
		if escape {
			escape = false
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if ch == '}' {
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					return text[start : i+1], nil
				}
			}
		}
	}
	return "", errors.New("no JSON object found in response")
}
