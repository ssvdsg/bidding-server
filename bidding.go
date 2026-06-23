// 招标管理模块
// 包含招标项目的查询、插入、删除、更新等核心功能
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func sanitizeBidRowForClient(row map[string]interface{}) map[string]interface{} {
	if row == nil {
		return nil
	}

	safe := make(map[string]interface{}, len(row))
	for key, value := range row {
		switch key {
		case "pdf_url", "original_pdf_url", "original_href", "html_url":
			continue
		default:
			safe[key] = value
		}
	}

	if value, ok := row["pdf_url"]; ok {
		url := strings.TrimSpace(fmt.Sprintf("%v", value))
		if url != "" && url != "<nil>" {
			safe["has_current_file"] = true
			safe["has_pdf_url"] = true
			safe["has_current_pdf"] = isLikelyPDFURLForClient(url)
			// 外部分享链接（如 WPS 灵析 kdocs.cn）不走本地代理，前端直接打开
			if external, label := classifyExternalShareURL(url); external {
				safe["current_is_external"] = true
				safe["current_external_url"] = url
				safe["current_external_label"] = label
			}
		}
	}
	if value, ok := row["original_pdf_url"]; ok {
		url := strings.TrimSpace(fmt.Sprintf("%v", value))
		if url != "" && url != "<nil>" {
			safe["has_original_file"] = true
			safe["has_original_pdf_url"] = true
			safe["has_original_pdf"] = isLikelyPDFURLForClient(url)
			// 仅当原始链接命中已知第三方平台白名单（如 WPS 灵析）时，才走外链直跳；
			// 普通远程 URL（如 cebpubservice 公告页）由"公告在线预览"承担展示，不再额外生成入口。
			if external, label := classifyExternalShareURL(url); external {
				safe["original_is_external"] = true
				safe["original_external_url"] = url
				safe["original_external_label"] = label
			}
		}
	}

	return safe
}

// classifyExternalShareURL 判断 URL 是否是已知的外部分享链接（不能本地代理或渲染，需直接打开）
// 返回 (是否外链, 友好平台名)
func classifyExternalShareURL(rawURL string) (bool, string) {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	switch {
	case strings.Contains(lower, "kdocs.cn/"), strings.Contains(lower, "kdocs.wps.cn"), strings.Contains(lower, "365.kdocs.cn"):
		return true, "WPS 灵析"
	case strings.Contains(lower, "docs.qq.com"):
		return true, "腾讯文档"
	case strings.Contains(lower, "shimo.im"):
		return true, "石墨文档"
	case strings.Contains(lower, "feishu.cn"), strings.Contains(lower, "larksuite.com"):
		return true, "飞书云文档"
	case strings.Contains(lower, "yuque.com"):
		return true, "语雀"
	}
	return false, ""
}

func isLikelyPDFURLForClient(rawURL string) bool {
	lowerURL := strings.ToLower(strings.TrimSpace(rawURL))
	if lowerURL == "" {
		return false
	}
	return strings.Contains(lowerURL, ".pdf") ||
		strings.HasPrefix(lowerURL, "/china/") ||
		strings.HasPrefix(lowerURL, "/files/")
}

// ========== 查询相关 ==========

// getBids 获取招标列表
// 支持分页、关键词搜索、地区筛选、来源筛选、AI分数筛选等功能
func getBids(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pageNum := parseInt(q.Get("pageNum"), 1)
	pageSize := parseInt(q.Get("pageSize"), 20)
	keyword := q.Get("keyword")
	region := q.Get("region")
	source := q.Get("source")
	minScore := parseInt(q.Get("minScore"), 0)
	publishDate := q.Get("publishDate")
	orderBy := q.Get("orderBy")
	if orderBy == "" {
		orderBy = "ai_score"
	}
	order := strings.ToLower(q.Get("order"))
	if order != "asc" {
		order = "desc"
	}

	conditions := []string{"(status = 0 OR status IS NULL OR status = 'interested')"}
	args := []interface{}{}

	if keyword != "" {
		conditions = append(conditions, "(title LIKE ? OR buyer LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}

	if region != "" {
		conditions = append(conditions, "(area LIKE ? OR city LIKE ?)")
		like := "%" + region + "%"
		args = append(args, like, like)
	}

	if source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, source)
	}

	if minScore == -1 {
		conditions = append(conditions, "ai_suitable = 0")
	} else if minScore > 0 {
		conditions = append(conditions, "ai_score >= ?")
		args = append(args, minScore)
	}

	if publishDate != "" {
		conditions = append(conditions, "DATE(FROM_UNIXTIME(publish_time)) = ?")
		args = append(args, publishDate)
	}

	validOrderFields := map[string]bool{
		"serial": true, "title": true, "buyer": true, "area": true, "budget": true,
		"bid_amount": true, "publish_time": true, "bid_end_time": true, "sign_end_time": true,
		"bid_open_time": true, "ai_score": true, "industry": true, "purchasing": true,
		"project_name": true, "china_bulletin_source": true, "china_left_open_bid_day": true,
		"zhibiao_procurement_method": true, "keywords": true, "zhibiao_credential": true,
	}
	orderField := "publish_time"
	if validOrderFields[orderBy] {
		orderField = orderBy
	}

	selectFields := `id, serial, title, buyer, area, city, district, budget, bid_amount, 
		buyer_person, buyer_tel, agency, agency_person, agency_tel, 
		publish_time, sign_end_time, bid_end_time, bid_open_time, 
		industry, site, source, project_code, project_name, 
		public_type, top_type, sub_type, purchasing, keywords, 
		deliver_area, deliver_city, 
		ai_suitable, ai_score, ai_match_level, ai_priority, ai_model,
		status, wechat_sent, wechat_sent_at, created_at, fetch_time,
		CASE WHEN ai_analysis IS NOT NULL AND ai_analysis != '' THEN '1' ELSE '' END as ai_analysis`

	query := "SELECT " + selectFields + " FROM bids WHERE " + strings.Join(conditions, " AND ")
	if orderField == "ai_score" || orderField == "publish_time" || orderField == "bid_end_time" || orderField == "sign_end_time" {
		query += fmt.Sprintf(" ORDER BY CASE WHEN %s IS NULL OR %s = 0 THEN 0 ELSE 1 END %s, %s %s", orderField, orderField, strings.ToUpper(order), orderField, strings.ToUpper(order))
		if orderField == "ai_score" {
			query += " , CASE WHEN source = 'china' THEN 1 ELSE 0 END DESC, CASE WHEN area LIKE '%云南%' OR city LIKE '%云南%' THEN 1 ELSE 0 END DESC, publish_time DESC"
		}
	} else {
		query += fmt.Sprintf(" ORDER BY %s %s", orderField, strings.ToUpper(order))
	}
	query += ", id DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (pageNum-1)*pageSize)

	rows, err := queryRowsContext(r.Context(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range rows {
		rows[i] = sanitizeBidRowForClient(rows[i])
	}

	countQuery := "SELECT COUNT(*) FROM bids WHERE " + strings.Join(conditions, " AND ")
	total, err := queryCountContext(r.Context(), countQuery, args[:len(args)-2]...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"list":  rows,
			"total": total,
		},
	})
}

// getBidDetail 获取招标详情
// 根据ID或编号查询招标项目的详细信息
func getBidDetail(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		serial = r.URL.Query().Get("id")
	}
	if serial == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 serial 或 id"})
		return
	}

	var row map[string]interface{}
	var err error
	if !strings.Contains(serial, "%") {
		rows, qErr := queryRows("SELECT * FROM bids WHERE serial = ? LIMIT 1", serial)
		if qErr == nil && len(rows) > 0 {
			row = rows[0]
		}
	}
	if row == nil {
		rows, qErr := queryRows("SELECT * FROM bids WHERE id = ? LIMIT 1", serial)
		if qErr == nil && len(rows) > 0 {
			row = rows[0]
		} else {
			err = qErr
		}
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "未找到数据"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       sanitizeBidRowForClient(row),
	})
}

// getTodayBiddings 获取今日招标列表
// 返回今天发布的招标项目列表
func getTodayBiddings(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayStartStr := todayStart.UTC().Format(time.RFC3339)

	query := `SELECT id, serial, title, buyer, area, city, budget, industry, 
	          publish_time, ai_suitable, ai_score, ai_match_level, ai_priority, ai_model,
	          source, status, created_at, keywords, project_name,
	          CASE WHEN ai_analysis IS NOT NULL AND ai_analysis != '' THEN '1' ELSE '' END as ai_analysis
	          FROM bids 
	          WHERE created_at >= ? AND (status = 0 OR status IS NULL)
	          ORDER BY created_at DESC`

	rows, err := queryRows(query, todayStartStr)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       rows,
	})
}

// getBiddingsByDateRange 按日期范围获取招标列表
// 根据指定的开始日期和结束日期查询招标项目
func getBiddingsByDateRange(w http.ResponseWriter, r *http.Request) {
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	if startDate == "" || endDate == "" {
		respondError(w, http.StatusBadRequest, "缺少开始日期或结束日期")
		return
	}

	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "开始日期格式错误")
		return
	}

	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "结束日期格式错误")
		return
	}

	startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, endTime.Location())

	startTimeStr := startTime.UTC().Format(time.RFC3339)
	endTimeStr := endTime.UTC().Format(time.RFC3339)

	query := `SELECT id, serial, title, buyer, area, city, budget, industry, 
	          publish_time, ai_suitable, ai_score, ai_match_level, ai_priority, 
	          source, status, created_at, keywords, project_name, description, detail 
	          FROM bids 
	          WHERE created_at >= ? AND created_at <= ? AND (status = 0 OR status IS NULL)
	          ORDER BY created_at DESC`

	rows, err := queryRows(query, startTimeStr, endTimeStr)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       rows,
	})
}

// searchBids 搜索招标项目
func searchBids(w http.ResponseWriter, r *http.Request) {
	var searchReq struct {
		Keyword string `json:"keyword"`
		Page    int    `json:"page"`
		Limit   int    `json:"limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&searchReq); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求")
		return
	}

	if searchReq.Page == 0 {
		searchReq.Page = 1
	}
	if searchReq.Limit == 0 {
		searchReq.Limit = 20
	}

	query := `SELECT id, serial, title, buyer, area, city, budget, industry, 
	          publish_time, ai_suitable, ai_score, ai_match_level, ai_priority, 
	          source, created_at FROM bids 
	          WHERE title LIKE ? OR buyer LIKE ?
	          ORDER BY created_at DESC LIMIT ? OFFSET ?`

	keyword := "%" + searchReq.Keyword + "%"
	offset := (searchReq.Page - 1) * searchReq.Limit

	rows, err := db.Query(query, keyword, keyword, searchReq.Limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	bids := []Bid{}
	for rows.Next() {
		var bid Bid
		err := rows.Scan(&bid.ID, &bid.Serial, &bid.Title, &bid.Buyer, &bid.Area,
			&bid.City, &bid.Budget, &bid.Industry, &bid.PublishTime, &bid.AISuitable,
			&bid.AIScore, &bid.AIMatchLevel, &bid.AIPriority, &bid.Source, &bid.CreatedAt)
		if err != nil {
			continue
		}
		bids = append(bids, bid)
	}

	respondJSON(w, http.StatusOK, bids)
}

// ========== 插入相关 ==========

// insertBidHandler 插入招标项目（建域平台）
func insertBidHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	data, err := decodeJSONBody(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	id := pickString(data, "id")
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 id"})
		return
	}
	safeID := encodeBidID(id)
	serial := pickString(data, "serial")
	if serial == "" {
		serial = fmt.Sprintf("JY-%s", time.Now().Format("20060102150405"))
	}
	title := pickString(data, "title")
	if title == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少标题"})
		return
	}
	source := pickString(data, "source")
	if source == "" {
		source = "jianyu"
	}
	fetchTime := normalizeFetchTime(pickString(data, "fetch_time"))
	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err = stmtInsertBid.ExecContext(context.Background(),
		safeID,
		serial,
		title,
		pickString(data, "buyer"),
		fetchTime,
		pickString(data, "area"),
		pickString(data, "city"),
		pickString(data, "district"),
		pickFloat(data, "budget"),
		pickFloat(data, "bid_amount"),
		pickString(data, "buyer_person", "buyerPerson"),
		pickString(data, "buyer_tel", "buyerTel"),
		pickString(data, "agency"),
		pickString(data, "agency_person", "agencyPerson"),
		pickString(data, "agency_tel", "agencyTel"),
		pickInt(data, "publish_time"),
		pickInt(data, "sign_end_time"),
		pickInt(data, "bid_end_time"),
		pickString(data, "detail"),
		pickString(data, "description"),
		pickString(data, "industry"),
		pickString(data, "site"),
		pickString(data, "original_href", "originalHref"),
		pickString(data, "pdf_url", "pdfUrl"),
		pickString(data, "html_url", "htmlUrl"),
		source,
		pickString(data, "project_code", "projectCode"),
		pickString(data, "project_name", "projectName"),
		pickString(data, "public_type", "publicType"),
		pickString(data, "top_type", "topType"),
		pickString(data, "sub_type", "subType"),
		pickString(data, "purchasing"),
		pickString(data, "keywords"),
		pickString(data, "deliver_area", "deliverArea"),
		pickString(data, "deliver_city", "deliverCity"),
		pickString(data, "deliver_detail", "deliverDetail"),
		createdAt,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": err.Error()})
		return
	}
	// 异步执行AI分析
	go func(id string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[AI分析] 项目 %s 分析时发生panic: %v", id, r)
			}
		}()

		enabled, err := isAutoAIAnalysisEnabled()
		if err != nil {
			log.Printf("[AI分析] 读取自动评分开关失败: %v", err)
			return
		}
		if !enabled {
			log.Printf("[AI分析] 自动评分开关关闭，跳过项目: %s", id)
			return
		}

		log.Printf("[AI分析] 开始异步分析项目: %s", id)
		record, err := fetchBidRecordByID(id)
		if err != nil {
			log.Printf("[AI分析] 获取项目 %s 记录失败: %v", id, err)
			return
		}

		result, err := performBidAnalysis(record, "", "auto", "")
		if err != nil {
			log.Printf("[AI分析] 项目 %s 分析失败: %v", id, err)
		} else {
			log.Printf("[AI分析] 项目 %s 分析完成，模型: %s", id, result.AIModel)

			// 发送微信通知
			go sendWechatAfterSingleAnalysis(record, result)
		}
	}(id)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"message": "成功",
			"id":      id,
			"serial":  serial,
		},
	})
}

// insertChinaBidHandler 插入中国招标投标公共服务平台的招标项目
func insertChinaBidHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	data, err := decodeJSONBody(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	bulletinUUID := pickString(data, "bulletinUUID")
	tenderName := pickString(data, "tenderProjectName")
	if bulletinUUID == "" || tenderName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少必要字段：bulletinUUID 或 tenderProjectName"})
		return
	}
	var existing string
	err = db.QueryRow("SELECT id FROM bids WHERE id = ?", bulletinUUID).Scan(&existing)
	if err == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 1, "error_msg": "记录已存在", "data": map[string]string{"id": bulletinUUID}})
		return
	}
	if err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	publishTime := parseTimeToUnix(pickString(data, "noticeSendTime"))
	bidEndTime := parseTimeToUnix(pickString(data, "bidOpenTime"))
	docGetEnd := parseTimeToUnix(pickString(data, "docGetEndTime"))
	bidDocEnd := parseTimeToUnix(pickString(data, "bidDocRefferEndTime"))
	serial := pickString(data, "bulletinID")
	detailURL := fmt.Sprintf("https://ctbpsp.com/#/bulletinDetail?uuid=%s", bulletinUUID)
	budget := 0.0
	if val := pickString(data, "winPrice", "budget", "budgetPrice"); val != "" {
		parsed := strings.ReplaceAll(val, ",", "")
		if f, parseErr := strconv.ParseFloat(parsed, 64); parseErr == nil {
			budget = f
		}
	}

	site := pickString(data, "bulletinSource", "site")
	if site == "" {
		site = "中国招投标公共服务平台"
	}

	potentialBidders := "[]"
	if val, ok := data["potential_bidders"]; ok {
		if b, err := json.Marshal(val); err == nil {
			potentialBidders = string(b)
		}
	}

	similarProjects := "[]"
	if val, ok := data["similar_projects"]; ok {
		if b, err := json.Marshal(val); err == nil {
			similarProjects = string(b)
		}
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err = stmtInsertChinaBid.ExecContext(context.Background(),
		bulletinUUID,
		serial,
		tenderName,
		pickString(data, "tenderBidder"),
		formattedFetchTime(),
		pickString(data, "reginProvince"),
		pickString(data, "regionName"),
		pickString(data, "district"),
		budget,
		pickFloat(data, "bid_amount", "bidAmount"),
		pickString(data, "buyer_person", "buyerPerson"),
		pickString(data, "buyer_tel", "buyerTel"),
		pickString(data, "agency"),
		pickString(data, "agency_person", "agencyPerson"),
		pickString(data, "agency_tel", "agencyTel"),
		publishTime,
		parseTimeToUnix(pickString(data, "signEndTime")),
		bidEndTime,
		pickString(data, "detail"),
		pickString(data, "description"),
		pickString(data, "industry"),
		site,
		detailURL,
		pickString(data, "pdfUrl", "pdf_url"),
		detailURL,
		"china",
		pickString(data, "projectCode", "project_code"),
		pickString(data, "projectName", "project_name"),
		pickString(data, "publicType", "public_type"),
		pickString(data, "topType", "top_type"),
		pickString(data, "subType", "sub_type"),
		pickString(data, "purchasing"),
		pickString(data, "keywords"),
		pickString(data, "deliverArea", "deliver_area"),
		pickString(data, "deliverCity", "deliver_city"),
		pickString(data, "deliverDetail", "deliver_detail"),
		createdAt,
		pickString(data, "bulletinID"),
		pickString(data, "noticeMedia"),
		pickString(data, "bulletinType"),
		pickString(data, "platformName"),
		pickString(data, "regionCode"),
		pickString(data, "bulletinSource"),
		pickString(data, "superviseDept"),
		pickString(data, "dataSource"),
		pickString(data, "classifyName"),
		pickString(data, "tenderAgency"),
		docGetEnd,
		bidDocEnd,
		pickString(data, "serverPlat"),
		pickString(data, "tradePlat"),
		pickString(data, "dataPlat"),
		pickInt(data, "leftOpenBidDay"),
		pickInt(data, "isNew"),
		tenderName,
		pickString(data, "bidSectionName"),
		pickString(data, "noticeName"),
		potentialBidders,
		similarProjects,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"message": "数据插入成功",
			"id":      bulletinUUID,
			"serial":  serial,
		},
	})
}

// insertZhibiaoBidHandler 插入指标平台的招标项目
func insertZhibiaoBidHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	data, err := decodeJSONBody(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	bidID := pickString(data, "id")
	title := pickString(data, "title")
	if bidID == "" || title == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少必要字段：id 或 title"})
		return
	}
	var existing string
	err = db.QueryRow("SELECT id FROM bids WHERE id = ?", bidID).Scan(&existing)
	if err == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 1, "error_msg": "记录已存在", "data": map[string]string{"id": bidID}})
		return
	}
	if err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	today := time.Now().Format("060102")
	serial := fmt.Sprintf("ZB-%s-%s", today, bidID)
	parseTime := func(key string) int64 {
		return parseTimeToUnix(pickString(data, key))
	}
	pdfURL := ""
	if filesVal, ok := data["files"].([]interface{}); ok {
		for _, fv := range filesVal {
			if fileMap, ok := fv.(map[string]interface{}); ok {
				name := strings.ToLower(pickString(fileMap, "name"))
				if strings.HasSuffix(name, ".pdf") {
					pdfURL = pickString(fileMap, "url")
					break
				}
			}
		}
	}
	publicType := pickString(data, "noticeType")
	if method := pickString(data, "procurementMethod"); method != "" {
		if publicType != "" {
			publicType = fmt.Sprintf("%s - %s", publicType, method)
		} else {
			publicType = method
		}
	}
	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err = stmtInsertZhibiaoBid.ExecContext(context.Background(),
		bidID,
		serial,
		title,
		pickString(data, "noticeUnit", "bidUnit"),
		formattedFetchTime(),
		pickString(data, "province"),
		pickFloat(data, "budgetPrice", "budget"),
		pickFloat(data, "bidPrice"),
		pickString(data, "agency"),
		pickString(data, "agencyContact"),
		pickString(data, "agencyPhone"),
		pickString(data, "bidContact"),
		pickString(data, "bidPhone"),
		parseTime("releaseTime"),
		parseTime("signEndTime"),
		parseTime("endTime"),
		parseTime("startTime"),
		pickString(data, "content"),
		pickString(data, "contentText"),
		pickString(data, "industry"),
		pickString(data, "keyWords"),
		pickString(data, "sourceUrl"),
		pdfURL,
		"zhibiao",
		pickString(data, "projectCode"),
		pickString(data, "projectName"),
		publicType,
		createdAt,
		pickString(data, "noticeType"),
		pickString(data, "procurementMethod"),
		pickFloat(data, "ceilPrice"),
		pickString(data, "credential"),
		pickString(data, "supplier"),
		pickString(data, "supplierDate"),
		pickString(data, "projectDemand"),
		pickString(data, "projectContent", "contentText"),
		pickString(data, "projectAddr"),
		pickString(data, "projectNode"),
		pickString(data, "bidType"),
		parseTime("signStartTime"),
		pickString(data, "planTime"),
		pickString(data, "highLightTitle"),
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"message":    "数据插入成功",
			"id":         bidID,
			"serial":     serial,
			"title":      title,
			"source":     "zhibiao",
			"publicType": publicType,
		},
	})
}

// ========== 删除和更新相关 ==========

// deleteBid 删除招标项目
func deleteBid(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少参数 id"})
		return
	}
	if _, err := db.Exec("DELETE FROM bids WHERE id = ?", req.ID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "删除失败: " + err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "message": "删除成功"})
}

// restoreBid 恢复招标项目
func restoreBid(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少参数 id"})
		return
	}
	if _, err := db.Exec("UPDATE bids SET status = 0 WHERE id = ?", req.ID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "恢复失败: " + err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "message": "恢复成功"})
}

// batchExclude 批量排除招标项目
func batchExclude(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少参数 ids 或 ids 为空"})
		return
	}
	placeholders := make([]string, len(req.IDs))
	args := make([]interface{}, len(req.IDs))
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("UPDATE bids SET status = 1 WHERE id IN (%s)", strings.Join(placeholders, ","))
	if _, err := db.Exec(query, args...); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "批量排除失败: " + err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "message": fmt.Sprintf("成功排除 %d 条记录", len(req.IDs)), "count": len(req.IDs)})
}

// batchDelete 批量删除招标项目
func batchDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少参数 ids 或 ids 为空"})
		return
	}
	placeholders := make([]string, len(req.IDs))
	args := make([]interface{}, len(req.IDs))
	for i, id := range req.IDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM bids WHERE id IN (%s)", strings.Join(placeholders, ","))
	if _, err := db.Exec(query, args...); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "批量删除失败: " + err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "message": fmt.Sprintf("成功删除 %d 条记录", len(req.IDs)), "count": len(req.IDs)})
}

// updateStatusHandler 更新招标项目状态
func updateStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		ID     string      `json:"id"`
		Status interface{} `json:"status"`
		Notes  string      `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "请求解析失败: " + err.Error()})
		return
	}
	if req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少参数 id"})
		return
	}

	var statusValue interface{}
	switch v := req.Status.(type) {
	case string:
		if v == "excluded" {
			statusValue = 1
		} else if v == "passed" {
			statusValue = 2
		} else if v == "interested" {
			statusValue = 0
		} else {
			statusValue = v
		}
	case float64:
		statusValue = int(v)
	default:
		statusValue = req.Status
	}

	if statusValue == nil {
		statusValue = 0
	}

	if _, err := db.Exec("UPDATE bids SET status = ? WHERE id = ?", statusValue, req.ID); err != nil {
		log.Printf("[updateStatus] 数据库更新失败: %v, id=%s, status=%v", err, req.ID, statusValue)
		respondError(w, http.StatusInternalServerError, "数据库更新失败: "+err.Error())
		return
	}

	log.Printf("[updateStatus] 成功更新状态: id=%s, status=%v", req.ID, statusValue)
	respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "data": map[string]string{"message": "状态更新成功"}})
}

// updatePDFHandler 更新PDF链接
func updatePDFHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		ID     string `json:"id"`
		Serial string `json:"serial"`
		PDFURL string `json:"pdf_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PDFURL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 pdf_url"})
		return
	}
	var result sql.Result
	var err error
	if req.ID != "" {
		result, err = db.Exec("UPDATE bids SET pdf_url = ? WHERE id = ?", req.PDFURL, req.ID)
	} else if req.Serial != "" {
		result, err = db.Exec("UPDATE bids SET pdf_url = ? WHERE serial = ?", req.PDFURL, req.Serial)
	} else {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少 id 或 serial"})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, _ := result.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "data": map[string]interface{}{"message": "pdf_url 已更新", "affected": rows}})
}

// ========== 潜在投标人和同类项目 ==========

// savePotentialBiddersHandler 保存潜在投标人
func savePotentialBiddersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		ID   string        `json:"id"`
		Data []interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少项目 ID"})
		return
	}
	if len(req.Data) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "数据格式错误，需要数组"})
		return
	}
	if _, err := db.Exec("UPDATE bids SET china_potential_bidders = ? WHERE id = ?", toJSONString(req.Data), req.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"message": "潜在投标人数据保存成功",
			"id":      req.ID,
			"count":   len(req.Data),
		},
	})
}
