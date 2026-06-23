// 微信通知模块
// 包含微信消息发送、项目通知、中标结果通知等功能
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func getDefaultWechatRoom() string {
	return getConfigValue("WECHAT_DEFAULT_ROOM", "")
}

func getHighScoreWechatRoom() string {
	return getConfigValue("WECHAT_HIGH_SCORE_ROOM", getDefaultWechatRoom())
}

func getHighScoreThreshold() int {
	raw := getConfigValue("WECHAT_HIGH_SCORE_THRESHOLD", "90")
	score, err := strconv.Atoi(raw)
	if err != nil || score <= 0 || score > 100 {
		return 90
	}
	return score
}

func getWechatNoticeBaseURL() string {
	baseURL := strings.TrimSpace(getConfigValue("WECHAT_NOTICE_BASE_URL", ""))
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return ""
	}
	return baseURL
}

func selectProjectWechatRoom(score int, suitable bool) string {
	if score >= getHighScoreThreshold() && suitable {
		return getHighScoreWechatRoom()
	}
	return getDefaultWechatRoom()
}

// ========== HTTP Handlers ==========

// sendWechatNotification 发送微信通知
func sendWechatNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SendID  string `json:"sendID"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.SendID == "" {
		req.SendID = getDefaultWechatRoom()
	}
	if req.Message == "" {
		respondError(w, http.StatusBadRequest, "消息内容不能为空")
		return
	}

	result, err := postWechatMessage(req.SendID, req.Message)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"result":  result,
	})
}

// sendProjectToWechatHandler 发送项目到微信
func sendProjectToWechatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req struct {
		ID     string `json:"id"`
		Serial string `json:"serial"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "无效的请求体",
		})
		return
	}

	if req.ID == "" && req.Serial == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "缺少必要参数: id 或 serial",
		})
		return
	}

	// 查询项目信息
	var query string
	var arg interface{}
	if req.ID != "" {
		query = "SELECT * FROM bids WHERE id = ?"
		arg = req.ID
	} else {
		query = "SELECT * FROM bids WHERE serial = ?"
		arg = req.Serial
	}

	rows, err := db.Query(query, arg)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "查询失败: " + err.Error(),
		})
		return
	}
	defer rows.Close()

	if !rows.Next() {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "项目不存在",
		})
		return
	}

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "获取列信息失败",
		})
		return
	}

	// 扫描结果
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "扫描结果失败",
		})
		return
	}

	// 构建项目map
	project := make(map[string]interface{})
	for i, col := range columns {
		var v interface{}
		val := values[i]
		b, ok := val.([]byte)
		if ok {
			v = string(b)
		} else {
			v = val
		}
		project[col] = v
	}

	// 解析AI分析结果
	var analysisResult ProjectAnalysisResult
	if aiAnalysis, ok := project["ai_analysis"].(string); ok && aiAnalysis != "" {
		if err := json.Unmarshal([]byte(aiAnalysis), &analysisResult); err != nil {
			log.Printf("解析AI分析结果失败: %v", err)
		}
	}

	bid := UnanalyzedBid{
		ID:                getString(project, "id"),
		Source:            getString(project, "source"),
		TenderProjectName: firstNonEmpty(getString(project, "tender_project_name"), getString(project, "title")),
		BidSectionName:    getString(project, "bid_section_name"),
		NoticeName:        getString(project, "notice_name"),
		ChinaClassifyName: getString(project, "china_classify_name"),
		Keywords:          getString(project, "keywords"),
		Area:              getString(project, "area"),
		City:              getString(project, "city"),
		Serial:            firstNonEmpty(getString(project, "serial"), req.Serial),
	}

	// 统一使用首次自动通知同一套格式
	wechatContent := formatProjectForWechat(bid, analysisResult)
	log.Printf("格式化后的微信消息: %s", wechatContent[:min(100, len(wechatContent))])

	// 按评分自动选择微信群
	roomID := selectProjectWechatRoom(analysisResult.Score, analysisResult.Suitable)
	result, err := postWechatMessage(roomID, wechatContent)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "发送失败: " + err.Error(),
		})
		return
	}

	if bid.ID != "" {
		if _, err := db.Exec("UPDATE bids SET wechat_sent = 1, wechat_sent_at = ? WHERE id = ?", time.Now().Format("2006-01-02 15:04:05"), bid.ID); err != nil {
			log.Printf("[微信通知] 更新手动发送状态失败: %v", err)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "项目已发送到微信群",
		"content":    wechatContent,
		"result":     result,
	})
}

// ========== 微信API调用 ==========

// postWechatMessage 发送微信消息
func postWechatMessage(wxid, msg string) (*WechatResponse, error) {
	url := getConfigValue("WECHAT_HOOK_URL", "")
	payload := WechatPayload{
		Type: "Q0001",
		Data: map[string]interface{}{
			"wxid": wxid,
			"msg":  msg,
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result WechatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 || result.Code != 200 {
		return &result, fmt.Errorf("wechat api error: %s", result.Msg)
	}
	return &result, nil
}

// ========== 消息格式化 ==========

// formatProjectForWechatFromMap 从map格式化项目信息为微信消息
func formatProjectForWechatFromMap(project map[string]interface{}, result ProjectAnalysisResult) string {
	var parts []string

	// 公告标题
	if title := getString(project, "tender_project_name"); title != "" {
		parts = append(parts, fmt.Sprintf("公告标题: %s", title))
	} else if title := getString(project, "title"); title != "" {
		parts = append(parts, fmt.Sprintf("公告标题: %s", title))
	}

	// 项目编号
	if serial := getString(project, "serial"); serial != "" {
		parts = append(parts, fmt.Sprintf("项目编号: %s", serial))
	}

	// 预算金额
	if budget := getFloat(project, "budget"); budget > 0 {
		if budget >= 10000 {
			parts = append(parts, fmt.Sprintf("预算金额: %.2f 万元", budget/10000))
		} else {
			parts = append(parts, fmt.Sprintf("预算金额: %.2f 元", budget))
		}
	}

	// 地区
	if area := getString(project, "area"); area != "" {
		parts = append(parts, fmt.Sprintf("地区: %s", area))
	}

	// 城市
	if city := getString(project, "city"); city != "" {
		parts = append(parts, fmt.Sprintf("城市: %s", city))
	}

	// 采购单位
	if buyer := getString(project, "buyer"); buyer != "" {
		parts = append(parts, fmt.Sprintf("采购单位: %s", buyer))
	}

	// 行业
	if industry := getString(project, "industry"); industry != "" {
		parts = append(parts, fmt.Sprintf("行业: %s", industry))
	}

	// AI分析结果
	if result.Score > 0 {
		parts = append(parts, fmt.Sprintf("\rAI评分: %d/100", result.Score))
	}
	if result.MatchLevel != "" {
		parts = append(parts, fmt.Sprintf("匹配度: %s", result.MatchLevel))
	}
	if result.Suitable {
		parts = append(parts, "适合度: 适合")
	} else {
		parts = append(parts, "适合度: 不适合")
	}
	if len(result.Reasons) > 0 {
		parts = append(parts, fmt.Sprintf("分析理由: %s", strings.Join(result.Reasons, "；")))
	}
	if result.Recommendation != "" {
		parts = append(parts, fmt.Sprintf("建议: %s", result.Recommendation))
	}

	// 使用 \r 作为换行符（微信格式）
	return strings.Join(parts, "\r")
}

// sendWinnerWechatNotification 发送中标结果微信通知
func sendWinnerWechatNotification(trackedBidID string, winnerInfo *WinnerInfo) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[微信通知] 发送中标结果时发生panic: %v", r)
		}
	}()

	// 获取项目信息
	var title, serial, room sql.NullString
	err := db.QueryRow(`SELECT title, serial, wechat_room_id FROM tracked_bids WHERE id = ?`, trackedBidID).Scan(&title, &serial, &room)
	if err != nil {
		log.Printf("[微信通知] 获取项目信息失败: %v", err)
		return
	}

	projectTitle := title.String
	if serial.Valid && serial.String != "" {
		projectTitle = fmt.Sprintf("[%s] %s", serial.String, projectTitle)
	}

	// 格式化微信消息
	message := formatWinnerInfoForWechat(projectTitle, winnerInfo)

	// 发送到微信（优先使用项目自定义微信群，其次使用默认群）
	roomID := getDefaultWechatRoom()
	if room.Valid && room.String != "" {
		roomID = room.String
	}
	result, err := postWechatMessage(roomID, message)
	if err != nil {
		log.Printf("[微信通知] 发送中标结果失败: %v", err)
		return
	}

	log.Printf("[微信通知] 中标结果已发送到微信群: 项目 %s", trackedBidID)
	_ = result
}

// formatWinnerInfoForWechat 格式化中标信息为微信消息
func formatWinnerInfoForWechat(projectTitle string, info *WinnerInfo) string {
	var msg strings.Builder
	// 根据是否有最终结果或仅有候选人，动态调整标题
	if info != nil && info.HasResult {
		msg.WriteString("【中标结果通知】\r")
	} else if info != nil && info.HasCandidate {
		msg.WriteString("【中标候选人公示】\r")
	} else {
		msg.WriteString("【中标信息通知】\r")
	}
	msg.WriteString(fmt.Sprintf("项目名称：%s\r", projectTitle))

	if info.Winner != "" {
		// 如果只有候选人阶段，这里通常是第一中标候选人
		if info.HasResult {
			msg.WriteString(fmt.Sprintf("中标单位：%s\r", info.Winner))
		} else {
			msg.WriteString(fmt.Sprintf("当前主候选人：%s\r", info.Winner))
		}
	}

	if info.WinnerAmount > 0 {
		if info.WinnerAmount >= 10000 {
			msg.WriteString(fmt.Sprintf("中标金额：%.2f 万元\r", info.WinnerAmount/10000))
		} else {
			msg.WriteString(fmt.Sprintf("中标金额：%.2f 元\r", info.WinnerAmount))
		}
	}

	if info.HasCandidate && info.CandidateNoticeTime != "" {
		msg.WriteString(fmt.Sprintf("候选人公告发布时间：%s\r", formatNoticeTimeForWechat(info.CandidateNoticeTime)))
	}
	if info.HasResult && info.ResultNoticeTime != "" {
		msg.WriteString(fmt.Sprintf("中标公告发布时间：%s\r", formatNoticeTimeForWechat(info.ResultNoticeTime)))
	}

	msg.WriteString("\r相关公告：\r")

	// 如果有候选人列表，在公告前补充一段列表信息
	if len(info.Candidates) > 0 {
		msg.WriteString("候选人名单：\r")
		for idx, name := range info.Candidates {
			msg.WriteString(fmt.Sprintf("%d. %s\r", idx+1, name))
		}
	}

	if info.TenderNoticeName != "" {
		msg.WriteString(fmt.Sprintf("招标公告：%s\r", info.TenderNoticeName))
		if info.TenderNoticeURL != "" {
			msg.WriteString(fmt.Sprintf("%s\r", info.TenderNoticeURL))
		}
	}

	if info.CandidateNoticeName != "" {
		msg.WriteString(fmt.Sprintf("中标候选人公示：%s\r", info.CandidateNoticeName))
		// 优先显示PDF链接
		if info.CandidatePDFURL != "" {
			// 如果已经是相对路径（包含 china/），添加完整URL前缀
			if strings.HasPrefix(info.CandidatePDFURL, "china/") || strings.HasPrefix(info.CandidatePDFURL, "/china/") {
				path := strings.TrimPrefix(info.CandidatePDFURL, "/")
				msg.WriteString(fmt.Sprintf("%s/%s\r", getWechatNoticeBaseURL(), path))
			} else {
				// 否则使用原始URL
				msg.WriteString(fmt.Sprintf("%s\r", info.CandidatePDFURL))
			}
		} else if info.CandidateNoticeURL != "" {
			msg.WriteString(fmt.Sprintf("%s\r", info.CandidateNoticeURL))
		}
	}

	if info.ResultNoticeName != "" {
		msg.WriteString(fmt.Sprintf("中标结果公告：%s\r", info.ResultNoticeName))
		// 优先显示PDF链接
		if info.ResultPDFURL != "" {
			// 如果已经是相对路径（包含 china/），添加完整URL前缀
			if strings.HasPrefix(info.ResultPDFURL, "china/") || strings.HasPrefix(info.ResultPDFURL, "/china/") {
				path := strings.TrimPrefix(info.ResultPDFURL, "/")
				msg.WriteString(fmt.Sprintf("%s/%s\r", getWechatNoticeBaseURL(), path))
			} else {
				// 否则使用原始URL
				msg.WriteString(fmt.Sprintf("%s\r", info.ResultPDFURL))
			}
		} else if info.ResultNoticeURL != "" {
			msg.WriteString(fmt.Sprintf("%s\r", info.ResultNoticeURL))
		}
	}

	return msg.String()
}

func formatNoticeTimeForWechat(original string) string {
	if original == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02T15:04:05",
		"2006/01/02 15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, original); err == nil {
			return t.Format("2006-01-02 15:04")
		}
	}
	return original
}
