// 核心业务逻辑文件
// 包含招标管理、AI分析、追踪中标、文件上传等核心功能
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func queryPotentialBidders(w http.ResponseWriter, r *http.Request) {
	rows, err := queryRows(`SELECT id, serial, title, china_potential_bidders, china_similar_projects
		FROM bids WHERE china_potential_bidders IS NOT NULL AND china_potential_bidders != ''
		ORDER BY created_at DESC LIMIT 10`)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"total":   len(rows),
			"records": rows,
		},
	})
}

func getExcludedBids(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := parseInt(q.Get("page"), 1)
	size := parseInt(q.Get("size"), 20)
	keyword := q.Get("keyword")
	region := q.Get("region")
	start := q.Get("exclude_start")
	end := q.Get("exclude_end")

	conditions := []string{"(status = 1 OR status = -1)"}
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
	if start != "" {
		conditions = append(conditions, "DATE(created_at) >= ?")
		args = append(args, start)
	}
	if end != "" {
		conditions = append(conditions, "DATE(created_at) <= ?")
		args = append(args, end)
	}

	// 只查询列表需要的字段
	selectFields := `id, serial, title, buyer, area, city, district, budget, bid_amount, 
		buyer_person, buyer_tel, agency, agency_person, agency_tel, 
		publish_time, sign_end_time, bid_end_time, bid_open_time, 
		industry, site, source, project_code, project_name, 
		public_type, top_type, sub_type, purchasing, keywords, 
		deliver_area, deliver_city, 
		ai_suitable, ai_score, ai_match_level, ai_priority, ai_model,
		status, wechat_sent, wechat_sent_at, created_at, fetch_time,
		CASE WHEN ai_analysis IS NOT NULL AND ai_analysis != '' THEN '1' ELSE '' END as ai_analysis,
		created_at AS excluded_time`

	query := "SELECT " + selectFields + " FROM bids WHERE " + strings.Join(conditions, " AND ") + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	argsWithLimit := append(append([]interface{}{}, args...), size, (page-1)*size)
	items, err := queryRows(query, argsWithLimit...)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": err.Error()})
		return
	}

	countQuery := "SELECT COUNT(*) FROM bids WHERE " + strings.Join(conditions, " AND ")
	total, err := queryCount(countQuery, args...)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"items": items,
			"total": total,
		},
	})
}

func getCompanyAwardSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := parseInt(q.Get("page"), 1)
	size := parseInt(q.Get("size"), 20)
	keyword := strings.TrimSpace(q.Get("keyword"))

	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	} else if size > 100 {
		size = 100
	}

	conditions := []string{"search_company IS NOT NULL", "search_company <> ''"}
	args := []interface{}{}
	if keyword != "" {
		conditions = append(conditions, "search_company LIKE ?")
		args = append(args, "%"+keyword+"%")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := "SELECT COUNT(DISTINCT search_company) FROM tender_announcements " + whereClause
	total, err := queryCount(countQuery, args...)
	if err != nil {
		if isTableMissing(err, "tender_announcements") {
			respondEmptyCompanySummary(w, page, size)
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "查询失败: " + err.Error()})
		return
	}

	argsWithLimit := append([]interface{}{}, args...)
	argsWithLimit = append(argsWithLimit, size, (page-1)*size)

	query := `
SELECT 
    ta.search_company,
    COUNT(*) AS total_projects,
    MAX(ta.notice_time) AS last_notice_time,
    (SELECT project_name FROM tender_announcements t2 WHERE t2.search_company = ta.search_company ORDER BY notice_time DESC, id DESC LIMIT 1) AS last_project_name,
    (SELECT win_bidder FROM tender_announcements t2 WHERE t2.search_company = ta.search_company ORDER BY notice_time DESC, id DESC LIMIT 1) AS last_win_bidder,
    (SELECT win_price FROM tender_announcements t2 WHERE t2.search_company = ta.search_company ORDER BY notice_time DESC, id DESC LIMIT 1) AS last_win_price
FROM tender_announcements ta
` + whereClause + `
GROUP BY ta.search_company
ORDER BY last_notice_time DESC
LIMIT ? OFFSET ?`

	items, err := queryRows(query, argsWithLimit...)
	if err != nil {
		if isTableMissing(err, "tender_announcements") {
			respondEmptyCompanySummary(w, page, size)
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "查询失败: " + err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"items": items,
			"total": total,
			"page":  page,
			"size":  size,
		},
	})
}

func getCompanyAwardRecords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	company := strings.TrimSpace(q.Get("company"))
	if company == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少参数 company"})
		return
	}

	page := parseInt(q.Get("page"), 1)
	size := parseInt(q.Get("size"), 20)
	keyword := strings.TrimSpace(q.Get("keyword"))

	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	} else if size > 100 {
		size = 100
	}

	conditions := []string{"search_company = ?"}
	args := []interface{}{company}
	if keyword != "" {
		conditions = append(conditions, "(project_name LIKE ? OR win_bidder LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	countQuery := "SELECT COUNT(*) FROM tender_announcements " + whereClause
	total, err := queryCount(countQuery, args...)
	if err != nil {
		if isTableMissing(err, "tender_announcements") {
			respondEmptyCompanyRecords(w, company, page, size)
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "查询失败: " + err.Error()})
		return
	}

	argsWithLimit := append([]interface{}{}, args...)
	argsWithLimit = append(argsWithLimit, size, (page-1)*size)

	query := `SELECT id, bulletin_id, project_name, win_bidder, win_price, notice_time, notice_url, details
	FROM tender_announcements ` + whereClause + `
	ORDER BY notice_time DESC, id DESC
	LIMIT ? OFFSET ?`

	items, err := queryRows(query, argsWithLimit...)
	if err != nil {
		if isTableMissing(err, "tender_announcements") {
			respondEmptyCompanyRecords(w, company, page, size)
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "查询失败: " + err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"company": company,
			"items":   items,
			"total":   total,
			"page":    page,
			"size":    size,
		},
	})
}

func getCompanyAwardDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少参数 id"})
		return
	}

	query := `SELECT id, bulletin_id, project_name, win_bidder, win_price, notice_time, notice_url, details, search_company
	FROM tender_announcements WHERE id = ? LIMIT 1`

	var record CompanyAwardRecord
	var details, searchCompany sql.NullString
	err := db.QueryRow(query, id).Scan(
		&record.ID,
		&record.BulletinID,
		&record.ProjectName,
		&record.WinBidder,
		&record.WinPrice,
		&record.NoticeTime,
		&record.NoticeURL,
		&details,
		&searchCompany,
	)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "记录不存在"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "查询失败: " + err.Error()})
		return
	}
	if details.Valid {
		record.Details = details.String
	}
	if searchCompany.Valid {
		record.SearchCompany = searchCompany.String
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": record})
}

func deleteChinaDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var count sql.NullInt64
	if err := db.QueryRow("SELECT COUNT(*) FROM bids WHERE source = 'china'").Scan(&count); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result, err := db.Exec("DELETE FROM bids WHERE source = 'china'")
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	deleted, _ := result.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"message":        "中国招标网数据删除成功",
			"deleted_count":  deleted,
			"original_count": count.Int64,
		},
	})
}

func executeSQLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.SQL) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少SQL语句"})
		return
	}
	sqlText := strings.ToUpper(strings.TrimSpace(body.SQL))
	allowed := []string{"ALTER TABLE", "CREATE INDEX", "DROP INDEX", "CREATE TABLE"}
	valid := false
	for _, prefix := range allowed {
		if strings.HasPrefix(sqlText, prefix) {
			valid = true
			break
		}
	}
	if !valid {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "只允许执行DDL语句（ALTER TABLE, CREATE INDEX等）"})
		return
	}

	// 黑名单：即使通过了 DDL 前缀检查，也拒绝危险的破坏性操作
	forbidden := []string{"DROP DATABASE", "DROP TABLE", "TRUNCATE TABLE", "DROP SCHEMA"}
	for _, kw := range forbidden {
		if strings.Contains(sqlText, kw) {
			respondJSON(w, http.StatusForbidden, map[string]interface{}{"error_code": 1, "error_msg": fmt.Sprintf("禁止执行危险操作: %s", kw)})
			return
		}
	}
	result, err := db.Exec(body.SQL)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": "SQL执行失败: " + err.Error()})
		return
	}
	rowsAffected, _ := result.RowsAffected()
	lastID, _ := result.LastInsertId()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"message":      "SQL执行成功",
			"rowsAffected": rowsAffected,
			"lastInsertId": lastID,
		},
	})
}

func workerStatisticsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	statusCondition := "(status = 0 OR status IS NULL OR status = 'interested')"
	todayCount, err := queryCount("SELECT COUNT(*) FROM bids WHERE DATE(created_at) = CURDATE()")
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	totalCount, err := queryCount("SELECT COUNT(*) FROM bids")
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	highPriority, err := queryCount(fmt.Sprintf("SELECT COUNT(*) FROM bids WHERE ai_priority = '高' AND %s", statusCondition))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	suitable, err := queryCount(fmt.Sprintf("SELECT COUNT(*) FROM bids WHERE ai_suitable = 1 AND %s", statusCondition))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	notSuitable, err := queryCount(fmt.Sprintf("SELECT COUNT(*) FROM bids WHERE (ai_score < 80 OR (ai_score >= 80 AND ai_suitable = 0)) AND %s", statusCondition))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	trend, err := queryRows(fmt.Sprintf(`
		SELECT DATE(created_at) AS date, COUNT(*) AS count
		FROM bids
		WHERE DATE(created_at) >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
		AND %s
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, statusCondition))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	regions, err := queryRows(fmt.Sprintf(`
		SELECT area, COUNT(*) AS count
		FROM bids
		WHERE area IS NOT NULL AND area != '' AND %s
		GROUP BY area
		ORDER BY count DESC
		LIMIT 10
	`, statusCondition))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"today":        todayCount,
			"total":        totalCount,
			"highPriority": highPriority,
			"suitable":     suitable,
			"notSuitable":  notSuitable,
			"trend":        trend,
			"regions":      regions,
		},
	})
}

func autoExcludeOldBidsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	fiveDaysAgo := time.Now().AddDate(0, 0, -5).Format("2006-01-02")
	rows, err := queryRows(`
		SELECT id, title, created_at
		FROM bids
		WHERE (status = 0 OR status IS NULL OR status = 'interested')
		AND DATE(created_at) <= ?
	`, fiveDaysAgo)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(rows) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"error_code": 0,
			"data":       map[string]interface{}{"message": "没有需要排除的招标", "excludedCount": 0, "cutoffDate": fiveDaysAgo},
		})
		return
	}
	if _, err := db.Exec(`
		UPDATE bids SET status = 1
		WHERE (status = 0 OR status IS NULL OR status = 'interested')
		AND DATE(created_at) <= ?
	`, fiveDaysAgo); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	samples := make([]string, 0, len(rows))
	for i, row := range rows {
		if i >= 5 {
			break
		}
		if title, ok := row["title"].(string); ok {
			samples = append(samples, title)
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"message":       "自动排除完成",
			"excludedCount": len(rows),
			"cutoffDate":    fiveDaysAgo,
			"sampleTitles":  samples,
		},
	})
}

func checkStatusDistributionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	statusRows, err := queryRows(`
		SELECT 
			CASE 
				WHEN status IS NULL THEN 'NULL'
				WHEN status = '' THEN 'EMPTY'
				ELSE status
			END AS status_value,
			COUNT(*) AS count
		FROM bids
		GROUP BY status
		ORDER BY count DESC
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recentRows, err := queryRows(`
		SELECT id, title, status, created_at, ai_score
		FROM bids
		ORDER BY id DESC
		LIMIT 10
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	excludedCount, err := queryCount("SELECT COUNT(*) FROM bids WHERE status = 1 OR status = -1")
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	excludedSamples, err := queryRows(`
		SELECT id, title, status, created_at
		FROM bids
		WHERE status = 1 OR status = -1
		LIMIT 5
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total, err := queryCount("SELECT COUNT(*) FROM bids")
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"totalRecords":       total,
			"statusDistribution": statusRows,
			"excludedCount":      excludedCount,
			"excludedSamples":    excludedSamples,
			"recentRecords":      recentRows,
		},
	})
}

func getPotentialBiddersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少项目 ID"})
		return
	}
	var payload sql.NullString
	if err := db.QueryRow("SELECT china_potential_bidders FROM bids WHERE id = ?", id).Scan(&payload); err != nil {
		if err == sql.ErrNoRows {
			respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "未找到该项目"})
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var data interface{}
	if payload.Valid && payload.String != "" {
		json.Unmarshal([]byte(payload.String), &data)
	}
	if data == nil {
		data = []interface{}{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "data": data})
}

func saveSimilarProjectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		ID       string        `json:"id"`
		Projects []interface{} `json:"projects"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少项目 ID"})
		return
	}
	if len(req.Projects) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "projects 必须是数组"})
		return
	}
	if _, err := db.Exec("UPDATE bids SET china_similar_projects = ? WHERE id = ?", toJSONString(req.Projects), req.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "data": map[string]interface{}{"message": "同类项目中标情况保存成功", "count": len(req.Projects)}})
}

func getSimilarProjectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少项目 ID"})
		return
	}
	var payload sql.NullString
	if err := db.QueryRow("SELECT china_similar_projects FROM bids WHERE id = ?", id).Scan(&payload); err != nil {
		if err == sql.ErrNoRows {
			respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "未找到该项目"})
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var data interface{}
	if payload.Valid && payload.String != "" {
		json.Unmarshal([]byte(payload.String), &data)
	}
	if data == nil {
		data = []interface{}{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": data, "errorMessage": ""})
}

// excludeAllBids 排除所有现有的招标文件
func excludeAllBids(w http.ResponseWriter, r *http.Request) {
	// 执行批量更新，将所有状态为0的招标设置为-1（排除）
	result, err := db.Exec("UPDATE bids SET status = -1 WHERE status = 0")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "排除失败: "+err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    fmt.Sprintf("成功排除 %d 条招标信息", affected),
		"data": map[string]interface{}{
			"affected": affected,
		},
	})
}

// excludeOldBidsByDays 根据指定天数排除旧招标
// days: 要排除多少天前的招标
func excludeOldBidsByDays(days int) (int64, error) {
	if days < 1 {
		days = 3 // 默认3天
	}

	// 计算指定天数前的时间
	targetDate := time.Now().AddDate(0, 0, -days)
	targetDateStr := targetDate.Format("2006-01-02")

	log.Printf("[EXCLUDE_OLD] 开始排除创建时间在 %d 天前（%s）的招标...", days, targetDateStr)

	// 执行更新：将 created_at 在指定天数前且状态为0的招标设置为-1（排除）
	// 使用 LEFT() 函数提取日期部分（前10个字符），然后与目标日期比较
	// 这样可以避免日期格式解析的问题，只比较日期部分，支持所有格式
	query := `UPDATE bids 
		SET status = -1 
		WHERE status = 0 
		AND created_at IS NOT NULL 
		AND created_at != '' 
		AND LENGTH(created_at) >= 10
		AND LEFT(created_at, 10) <= ?`

	result, err := db.Exec(query, targetDateStr)
	if err != nil {
		log.Printf("[EXCLUDE_OLD] 排除失败: %v", err)
		return 0, err
	}

	affected, _ := result.RowsAffected()
	log.Printf("[EXCLUDE_OLD] 成功排除 %d 条 %d 天前的招标信息", affected, days)
	return affected, nil
}

// autoExcludeOldBids 自动排除抓取时间在指定天数前的招标
// 这个函数会被定时任务调用，每天运行一次
func autoExcludeOldBids() {
	// 检查配置是否启用
	var enabled string
	err := db.QueryRow("SELECT value FROM configs WHERE `key` = ?", "auto_exclude_old_bids_enabled").Scan(&enabled)
	if err == sql.ErrNoRows || enabled != "true" {
		log.Println("[AUTO_EXCLUDE] 自动排除旧招标功能未启用，跳过执行")
		return
	}
	if err != nil && err != sql.ErrNoRows {
		log.Printf("[AUTO_EXCLUDE] 读取配置失败: %v", err)
		return
	}

	// 读取配置的天数
	var daysStr string
	days := 3 // 默认3天
	err = db.QueryRow("SELECT value FROM configs WHERE `key` = ?", "auto_exclude_days").Scan(&daysStr)
	if err == nil && daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays >= 1 && parsedDays <= 30 {
			days = parsedDays
		}
	}

	affected, err := excludeOldBidsByDays(days)
	if err != nil {
		log.Printf("[AUTO_EXCLUDE] 自动排除失败: %v", err)
	} else {
		log.Printf("[AUTO_EXCLUDE] 自动排除完成，共排除 %d 条招标", affected)
	}
}

func deleteOldBidsByDays(days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}

	targetDate := time.Now().AddDate(0, 0, -days)
	targetDateStr := targetDate.Format("2006-01-02")
	log.Printf("[DELETE_OLD] 开始删除创建时间在 %d 天前（%s）的招标...", days, targetDateStr)

	query := `DELETE FROM bids
		WHERE created_at IS NOT NULL
		AND created_at != ''
		AND LENGTH(created_at) >= 10
		AND (
			status IN (1, -1)
			OR ai_suitable = 0
		)
		AND LEFT(created_at, 10) <= ?`

	result, err := db.Exec(query, targetDateStr)
	if err != nil {
		log.Printf("[DELETE_OLD] 删除失败: %v", err)
		return 0, err
	}

	affected, _ := result.RowsAffected()
	log.Printf("[DELETE_OLD] 成功删除 %d 条 %d 天前的招标信息", affected, days)
	return affected, nil
}

func autoDeleteOldBids() {
	var daysStr string
	err := db.QueryRow("SELECT value FROM configs WHERE `key` = ?", "auto_delete_days").Scan(&daysStr)
	if err == sql.ErrNoRows {
		log.Println("[AUTO_DELETE] 未配置自动删除天数，跳过执行")
		return
	}
	if err != nil {
		log.Printf("[AUTO_DELETE] 读取配置失败: %v", err)
		return
	}

	days, err := strconv.Atoi(strings.TrimSpace(daysStr))
	if err != nil || days <= 0 {
		log.Printf("[AUTO_DELETE] auto_delete_days=%q，无需执行", daysStr)
		return
	}

	affected, err := deleteOldBidsByDays(days)
	if err != nil {
		log.Printf("[AUTO_DELETE] 自动删除失败: %v", err)
	} else {
		log.Printf("[AUTO_DELETE] 自动删除完成，共删除 %d 条招标", affected)
	}
}

// manualExcludeOldBidsHandler 手动执行排除旧招标的API处理器
func manualExcludeOldBidsHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Days int `json:"days"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求: "+err.Error())
		return
	}

	if req.Days < 1 || req.Days > 30 {
		respondError(w, http.StatusBadRequest, "天数必须在1-30之间")
		return
	}

	affected, err := excludeOldBidsByDays(req.Days)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "排除失败: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    fmt.Sprintf("成功排除 %d 条招标信息", affected),
		"data": map[string]interface{}{
			"affected": affected,
			"days":     req.Days,
		},
	})
}

// 辅助函数：从map获取字符串
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// 辅助函数：从map获取整数
func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		case string:
			if i, err := strconv.Atoi(val); err == nil {
				return i
			}
		}
	}
	return 0
}

// 辅助函数：从map获取int64
func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case int64:
			return val
		case int:
			return int64(val)
		case float64:
			return int64(val)
		case string:
			if i, err := strconv.ParseInt(val, 10, 64); err == nil {
				return i
			}
		}
	}
	return 0
}

// 辅助函数：从map获取浮点数
func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int:
			return float64(val)
		case int64:
			return float64(val)
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f
			}
		}
	}
	return 0
}

// 获取近5天的UUID列表
func getExistingUUIDs(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	conditions := []string{"created_at IS NOT NULL", "created_at >= DATE_SUB(NOW(), INTERVAL 5 DAY)"}
	args := []interface{}{}
	if source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, source)
	}

	query := "SELECT id FROM bids WHERE " + strings.Join(conditions, " AND ")
	rows, err := db.Query(query, args...)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error_code": 1, "error_msg": err.Error()})
		return
	}
	defer rows.Close()

	uuids := make([]string, 0)
	for rows.Next() {
		var id sql.NullString
		if err := rows.Scan(&id); err == nil && id.Valid {
			uuids = append(uuids, id.String)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"uuids": uuids,
			"total": len(uuids),
		},
	})
}

func maxSerialHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 date 参数"})
		return
	}
	var serial sql.NullString
	err := db.QueryRow("SELECT serial FROM bids WHERE serial LIKE ? ORDER BY serial DESC LIMIT 1", date+"-%").Scan(&serial)
	if err != nil && err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"maxSerial": serial.String})
}

func getRecentIDsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	days := parseInt(r.URL.Query().Get("days"), 7)
	if days <= 0 {
		days = 7
	}
	query := "SELECT id FROM bids WHERE created_at >= DATE_SUB(NOW(), INTERVAL ? DAY)"
	rows, err := db.Query(query, days)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id sql.NullString
		if err := rows.Scan(&id); err == nil && id.Valid {
			ids = append(ids, id.String)
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ids":  ids,
		"days": days,
	})
}

func getTodaySerialsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 date 参数"})
		return
	}
	rows, err := db.Query("SELECT serial FROM bids WHERE serial LIKE ?", date+"-%")
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	serials := make([]string, 0)
	for rows.Next() {
		var serial sql.NullString
		if err := rows.Scan(&serial); err == nil && serial.Valid {
			serials = append(serials, serial.String)
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"serials": serials})
}

func checkTitleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少标题"})
		return
	}
	var source sql.NullString
	err := db.QueryRow("SELECT source FROM bids WHERE title = ? LIMIT 1", req.Title).Scan(&source)
	if err != nil && err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"exists": err != sql.ErrNoRows,
		"source": source.String,
	})
}

func checkSerialHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		Serial string `json:"serial"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Serial == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 serial"})
		return
	}
	var serial sql.NullString
	err := db.QueryRow("SELECT serial FROM bids WHERE serial = ? LIMIT 1", req.Serial).Scan(&serial)
	if err != nil && err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"exists": err != sql.ErrNoRows})
}

func checkDuplicatesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "ids 参数必须是非空数组"})
		return
	}
	countMap := make(map[string]int)
	encodedSet := make(map[string]struct{})
	for _, id := range req.IDs {
		normalized := decodeBidID(id)
		if normalized == "" {
			continue
		}
		countMap[normalized]++
		encoded := encodeBidID(normalized)
		if encoded != "" {
			encodedSet[encoded] = struct{}{}
		}
	}
	if len(encodedSet) > 0 {
		uniqueEncoded := make([]string, 0, len(encodedSet))
		for encoded := range encodedSet {
			uniqueEncoded = append(uniqueEncoded, encoded)
		}
		const batchSize = 100
		for i := 0; i < len(uniqueEncoded); i += batchSize {
			end := i + batchSize
			if end > len(uniqueEncoded) {
				end = len(uniqueEncoded)
			}
			batch := uniqueEncoded[i:end]
			placeholders := strings.Repeat("?,", len(batch))
			placeholders = strings.TrimSuffix(placeholders, ",")
			query := fmt.Sprintf("SELECT id FROM bids WHERE id IN (%s)", placeholders)
			args := make([]interface{}, len(batch))
			for idx, v := range batch {
				args[idx] = v
			}
			rows, err := db.Query(query, args...)
			if err != nil {
				respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for rows.Next() {
				var stored sql.NullString
				if err := rows.Scan(&stored); err == nil && stored.Valid {
					decoded := decodeBidID(stored.String)
					countMap[decoded]++
				}
			}
			rows.Close()
		}
	}
	duplicates := make(map[string]int)
	for id, count := range countMap {
		if count > 1 {
			duplicates[id] = count
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"duplicates": duplicates})
}

func checkIDsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "ids 参数必须是非空数组"})
		return
	}
	encodedMap := make(map[string]string, len(req.IDs))
	uniqueSet := make(map[string]struct{})
	for _, id := range req.IDs {
		safe := encodeBidID(id)
		encodedMap[id] = safe
		if safe != "" {
			uniqueSet[safe] = struct{}{}
		}
	}
	const batchSize = 200
	found := make(map[string]struct{})
	if len(uniqueSet) > 0 {
		unique := make([]string, 0, len(uniqueSet))
		for v := range uniqueSet {
			unique = append(unique, v)
		}
		for i := 0; i < len(unique); i += batchSize {
			end := i + batchSize
			if end > len(unique) {
				end = len(unique)
			}
			batch := unique[i:end]
			placeholders := strings.Repeat("?,", len(batch))
			placeholders = strings.TrimSuffix(placeholders, ",")
			query := fmt.Sprintf("SELECT id FROM bids WHERE id IN (%s)", placeholders)
			args := make([]interface{}, len(batch))
			for idx, v := range batch {
				args[idx] = v
			}
			rows, err := db.Query(query, args...)
			if err != nil {
				respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for rows.Next() {
				var id sql.NullString
				if err := rows.Scan(&id); err == nil && id.Valid {
					found[id.String] = struct{}{}
				}
			}
			rows.Close()
		}
	}
	existsMap := make(map[string]bool, len(req.IDs))
	for _, id := range req.IDs {
		safe := encodedMap[id]
		_, ok := found[safe]
		existsMap[id] = ok
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"existsMap": existsMap})
}

func checkIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "id 参数不能为空"})
		return
	}
	safeID := encodeBidID(req.ID)
	var id sql.NullString
	err := db.QueryRow("SELECT id FROM bids WHERE id = ?", safeID).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"exists": err != sql.ErrNoRows})
}

func getScoreFromAnalysis(analysis map[string]interface{}) int {
	if v, ok := analysis["score"]; ok {
		return toInt(v)
	}
	if v, ok := analysis["Score"]; ok {
		return toInt(v)
	}
	if v, ok := analysis["SCORE"]; ok {
		return toInt(v)
	}
	return 0
}

func fallbackAnalysis(reason, raw string) map[string]interface{} {
	analysis := map[string]interface{}{
		"suitable":       false,
		"score":          50,
		"matchLevel":     "需要人工审核",
		"reasons":        []string{reason},
		"advantages":     []string{},
		"risks":          []string{},
		"recommendation": "请人工审核该项目",
		"priority":       "中",
	}
	if raw != "" {
		analysis["rawResponse"] = strings.TrimSpace(raw)
	}
	return analysis
}

func toBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "true" || lower == "1" || lower == "是"
	case float64:
		return v != 0
	case json.Number:
		i, _ := v.Int64()
		return i != 0
	default:
		return false
	}
}

func toInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(math.Round(v))
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
		if f, err := v.Float64(); err == nil {
			return int(math.Round(f))
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return int(math.Round(f))
		}
	}
	return 0
}

func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func toStringSlice(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		slice := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(toString(item)); s != "" {
				slice = append(slice, s)
			}
		}
		return slice
	case string:
		if strings.TrimSpace(v) == "" {
			return []string{}
		}
		return []string{strings.TrimSpace(v)}
	default:
		return []string{}
	}
}

func updateBidAIFields(bidID string, analysis map[string]interface{}, prompt, thinking, usedModel string) (string, error) {
	if bidID == "" {
		return "", errors.New("bid id required")
	}
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		analysisJSON = []byte("{}")
	}
	analyzedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`
		UPDATE bids SET
			ai_analysis = ?,
			ai_suitable = ?,
			ai_score = ?,
			ai_match_level = ?,
			ai_priority = ?,
			ai_reasons = ?,
			ai_advantages = ?,
			ai_risks = ?,
			ai_recommendation = ?,
			ai_prompt = ?,
			ai_thinking_process = ?,
			ai_model = ?,
			analyzed_at = ?
		WHERE id = ?
	`,
		string(analysisJSON),
		boolToInt(toBool(analysis["suitable"])),
		getScoreFromAnalysis(analysis),
		defaultString(toString(analysis["matchLevel"]), "需要人工审核"),
		defaultString(toString(analysis["priority"]), "中"),
		toJSONString(toStringSlice(analysis["reasons"])),
		toJSONString(toStringSlice(analysis["advantages"])),
		toJSONString(toStringSlice(analysis["risks"])),
		defaultString(toString(analysis["recommendation"]), ""),
		prompt,
		thinking,
		usedModel,
		analyzedAt,
		bidID,
	)
	return analyzedAt, err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func evaluateBidEligibility(title, detail, purchasing, keywords, description string) (bool, []string) {
	titleLower := strings.ToLower(title)
	detailLower := strings.ToLower(truncateRunes(detail, 500))
	reasons := make([]string, 0)
	for _, keyword := range aiExcludeKeywords {
		kw := strings.ToLower(keyword)
		if strings.Contains(titleLower, kw) || strings.Contains(detailLower, kw) {
			reasons = append(reasons, "项目涉及本公司不承接的业务类型（危险品、建筑、装修等）")
			return true, reasons
		}
	}
	combined := strings.ToLower(strings.Join([]string{title, purchasing, keywords, description}, " "))
	isTransport := false
	for _, keyword := range transportKeywords {
		if strings.Contains(combined, strings.ToLower(keyword)) {
			isTransport = true
			break
		}
	}
	if !isTransport {
		reasons = append(reasons, "非运输/物流/货运相关项目，不符合公司业务范围")
		return true, reasons
	}
	return false, reasons
}

func buildEarlyExitAnalysis(reasons []string) map[string]interface{} {
	if len(reasons) == 0 {
		reasons = []string{"项目不符合公司业务范围"}
	}
	return map[string]interface{}{
		"suitable":   false,
		"score":      0,
		"matchLevel": "完全不匹配",
		"reasons":    reasons,
		"advantages": []string{},
		"risks": []string{
			"业务类型不在公司经营范围内",
			"不符合车队运输业务",
		},
		"recommendation": "不建议投标，该项目不在公司业务范围内",
		"priority":       "低",
		"dimensionScores": map[string]int{
			"业务类型判断":  0,
			"地理位置适配度": 0,
			"行业相关性":   0,
			"项目规模契合度": 0,
		},
	}
}

func callKimiFormatter(content string) (string, string, error) {
	systemPrompt := `你是一名专业的文档排版助手，擅长将混乱的招标公告内容整理成结构清晰、易于阅读的 HTML 格式。

**你的任务**：
1. 理解并分析原始招标内容
2. 提取关键信息并按逻辑结构重新组织
3. 使用 HTML 格式输出，层次分明
4. **保留所有原始内容，不增加、不删除、不修改任何实质性信息**
5. 只调整格式和结构，使其更易阅读

**输出要求**：
- 使用 HTML 格式输出
- 使用 <h2> 作为一级标题，<h3> 作为二级标题
- 重要信息使用 <strong> 标签加粗
- 列表使用 <ul><li> 或 <ol><li>
- 段落使用 <p> 标签
- 保持专业、简洁的语言风格
- **禁止删除任何原始信息**
- **禁止添加不存在的信息**
- **禁止修改数字、日期、联系方式等关键数据**
- 不要添加 <html>, <head>, <body> 等外层标签，只输出内容部分

**标准结构参考**（根据实际内容调整）：
<h2>项目基本信息</h2>
<h2>采购需求</h2>
<h2>投标人资格要求</h2>
<h2>重要时间节点</h2>
<h2>联系方式</h2>
<h2>其他说明</h2>`

	log.Printf("[AI排版] 开始调用 External AI")

	// 使用与AI分析相同的唯一中转AI接口
	formatted, _, err := callRelayStreamingAI(systemPrompt, content)
	if err != nil {
		log.Printf("[AI排版] Relay AI 调用失败: %v", err)
		return "", "", fmt.Errorf("AI排版失败: %v", err)
	}

	// 移除可能的 <think> 标签
	if strings.Contains(formatted, "</think>") {
		formatted = strings.Split(formatted, "</think>")[1]
	}
	formatted = strings.TrimSpace(formatted)

	log.Printf("[AI排版] 排版完成，长度: %d", len(formatted))
	return formatted, fmt.Sprintf("Relay AI (%s)", getConfigValue("RELAY_AI_MODEL", defaultRelayAIModel)), nil
}

// batchAnalyzeUnprocessedBids 批量分析未处理的招标项目
func batchAnalyzeUnprocessedBids() {
	enabled, err := isAutoAIAnalysisEnabled()
	if err != nil {
		log.Printf("[CRON] 读取自动AI评分开关失败: %v", err)
		return
	}
	if !enabled {
		log.Println("[CRON] 自动AI评分开关关闭，跳过本轮检查")
		return
	}

	batchAIState.mu.Lock()
	if batchAIState.running {
		// 饥饿兜底：若 running 超过硬过期时间，强制复位（防止残留 panic 导致永久卡死）
		if time.Since(batchAIState.lastStartedAt) > batchAIHardExpiry {
			log.Printf("[CRON] 批量AI分析任务已超过 %v，强制复位 running 标志", batchAIHardExpiry)
			batchAIState.running = false
		} else {
			batchAIState.mu.Unlock()
			log.Println("[CRON] 批量AI分析任务仍在运行，跳过本轮检查")
			return
		}
	}
	batchAIState.running = true
	batchAIState.lastStartedAt = time.Now()
	batchAIState.mu.Unlock()

	defer func() {
		batchAIState.mu.Lock()
		batchAIState.running = false
		batchAIState.mu.Unlock()
		if r := recover(); r != nil {
			log.Printf("[CRON] 批量分析任务发生panic: %v", r)
		}
	}()

	// 查询所有未分析的项目，限制每次处理10个
	// 双重保险：忽略已排除（status=1/-1）的项目，避免被排除的项目仍被 AI 评分
	query := `
    SELECT id
    FROM bids
    WHERE (ai_analysis IS NULL OR ai_analysis = '')
      AND (status IS NULL OR status = 0)
    ORDER BY created_at
    LIMIT 10;
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("[CRON] 查询未分析项目失败: %v", err)
		return
	}
	defer rows.Close()

	var bids []UnanalyzedBid
	for rows.Next() {
		var bid UnanalyzedBid
		err := rows.Scan(
			&bid.ID,
		)
		if err != nil {
			log.Printf("[CRON] 扫描项目数据失败: %v", err)
			continue
		}
		bids = append(bids, bid)
	}

	if len(bids) == 0 {
		log.Println("[CRON] 没有需要分析的项目")
		return
	}
	log.Printf("[CRON] 找到 %d 个未分析项目，开始批量AI分析...", len(bids))
	const perItemTimeout = 3 * time.Minute
	for _, bid := range bids {
		log.Printf("[CRON] 待分析项目ID: %s", bid.ID)
		log.Printf("[AI分析] 开始异步分析项目: %s", bid.ID)
		record, err := fetchBidRecordByID(bid.ID)
		if err != nil {
			log.Printf("[AI分析] 获取项目 %s 记录失败: %v", bid.ID, err)
			return
		}

		// 单项目 3 分钟硬超时 — 防止单个 AI 调用卡死阻塞整批
		type analysisOutcome struct {
			result *aiComputationResult
			err    error
		}
		done := make(chan analysisOutcome, 1)
		go func() {
			r, e := performBidAnalysis(record, "", "auto", "")
			done <- analysisOutcome{r, e}
		}()
		select {
		case outcome := <-done:
			if outcome.err != nil {
				log.Printf("[AI分析] 项目 %s 分析失败: %v", bid.ID, outcome.err)
			} else {
				log.Printf("[AI分析] 项目 %s 分析完成，模型: %s", bid.ID, outcome.result.AIModel)
				go sendWechatAfterSingleAnalysis(record, outcome.result)
			}
		case <-time.After(perItemTimeout):
			log.Printf("[AI分析] 项目 %s 超时 (%v)，跳过", bid.ID, perItemTimeout)
		}
	}

}

func isAutoAIAnalysisEnabled() (bool, error) {
	var enabled string
	err := db.QueryRow("SELECT value FROM configs WHERE `key` = ?", "AUTO_AI_ANALYSIS_ENABLED").Scan(&enabled)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(strings.ToLower(enabled)) == "true", nil
}

// sendWechatAfterSingleAnalysis 单个项目分析后发送微信通知
func sendWechatAfterSingleAnalysis(record *BidRecord, result *aiComputationResult) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[微信通知] 发送时发生panic: %v", r)
		}
	}()

	// 解析AI分析结果
	var analysisResult ProjectAnalysisResult
	if result.Analysis != nil {
		analysisResult.Score = getScoreFromAnalysis(result.Analysis)
		analysisResult.Suitable = toBool(result.Analysis["suitable"])
		analysisResult.MatchLevel = toString(result.Analysis["matchLevel"])
		analysisResult.Priority = toString(result.Analysis["priority"])
		analysisResult.Reasons = toStringSlice(result.Analysis["reasons"])
		analysisResult.Advantages = toStringSlice(result.Analysis["advantages"])
		analysisResult.Risks = toStringSlice(result.Analysis["risks"])
		analysisResult.Recommendation = toString(result.Analysis["recommendation"])
	}

	// 构建UnanalyzedBid
	bid := UnanalyzedBid{
		ID:                record.ID,
		Source:            nullStringValue(record.Source),
		TenderProjectName: nullStringValue(record.ChinaTenderProjectName),
		BidSectionName:    nullStringValue(record.ChinaBidSectionName),
		NoticeName:        nullStringValue(record.ChinaNoticeName),
		ChinaClassifyName: nullStringValue(record.ChinaClassifyName),
		Keywords:          nullStringValue(record.Keywords),
		Area:              nullStringValue(record.Area),
		City:              nullStringValue(record.City),
		Serial:            nullStringValue(record.Serial),
	}

	// 如果标题为空，使用title字段
	if bid.TenderProjectName == "" {
		bid.TenderProjectName = nullStringValue(record.Title)
	}

	// 发送微信通知
	if err := sendWechatAfterAnalysis(bid, analysisResult); err != nil {
		log.Printf("[微信通知] 发送失败: %v", err)
	}
}

// sendWechatAfterAnalysis 在AI分析后发送微信通知
func sendWechatAfterAnalysis(bid UnanalyzedBid, result ProjectAnalysisResult) error {
	roomID := selectProjectWechatRoom(result.Score, result.Suitable)
	if roomID == getHighScoreWechatRoom() && roomID != getDefaultWechatRoom() {
		log.Printf("[CRON] 检测到高分项目（%d分），准备发送微信通知到高分群...", result.Score)
	} else {
		log.Printf("[CRON] 项目得分 %d 分，发送到默认群", result.Score)
	}

	// 格式化消息内容
	message := formatProjectForWechat(bid, result)

	// 发送微信消息
	_, err := postWechatMessage(roomID, message)
	if err != nil {
		return fmt.Errorf("发送微信消息失败: %v", err)
	}

	// 更新数据库标记为已发送
	updateQuery := `UPDATE bids SET wechat_sent = 1, wechat_sent_at = ? WHERE id = ?`
	_, err = db.Exec(updateQuery, time.Now().Format("2006-01-02 15:04:05"), bid.ID)
	if err != nil {
		log.Printf("[CRON] 更新微信发送状态失败: %v", err)
	} else {
		log.Printf("[CRON] 微信通知发送成功: 项目 %s", bid.ID)
	}

	return nil
}

// formatProjectForWechat 格式化项目信息为微信消息
func formatProjectForWechat(bid UnanalyzedBid, result ProjectAnalysisResult) string {
	var parts []string

	// 公告标题
	if bid.TenderProjectName != "" {
		parts = append(parts, fmt.Sprintf("公告标题: %s", bid.TenderProjectName))
	}

	// AI 评分
	if result.Score > 0 {
		parts = append(parts, fmt.Sprintf("AI评分: %d分", result.Score))
	}

	// 查询完整的bid信息以获取更多字段
	fullBid, err := fetchFullBidInfo(bid.ID)
	if err == nil {
		// 预算金额
		if fullBid.Budget > 0 {
			parts = append(parts, fmt.Sprintf("预算金额: %s", convertAmount(fullBid.Budget)))
		}

		// 发布网站
		if fullBid.Site != "" {
			parts = append(parts, fmt.Sprintf("发布网站: %s", fullBid.Site))
		}

		// 采购单位
		if fullBid.Buyer != "" {
			parts = append(parts, fmt.Sprintf("采购单位: %s", fullBid.Buyer))
		}

		// 行业分类
		if fullBid.Industry != "" {
			parts = append(parts, fmt.Sprintf("行业分类: %s", fullBid.Industry))
		}

		// 发布时间
		if fullBid.PublishTime > 0 {
			timeStr := time.Unix(fullBid.PublishTime, 0).Format("2006-01-02 15:04:05")
			parts = append(parts, fmt.Sprintf("发布时间: %s", timeStr))
		}

		// 投标截止时间
		if fullBid.BidEndTime > 0 {
			timeStr := time.Unix(fullBid.BidEndTime, 0).Format("2006-01-02 15:04:05")
			parts = append(parts, fmt.Sprintf("投标截止: %s", timeStr))
		}
	}

	// 地区
	if bid.Area != "" || bid.City != "" {
		parts = append(parts, fmt.Sprintf("地区: %s - %s", bid.Area, bid.City))
	}

	// 关键词
	if bid.Keywords != "" {
		parts = append(parts, fmt.Sprintf("关键词: %s", bid.Keywords))
	}

	// 详情链接
	detailID := bid.ID
	if detailID == "" {
		detailID = bid.Serial
	}
	detailURL := fmt.Sprintf("%s/bids/%s", getWechatNoticeBaseURL(), detailID)
	parts = append(parts, fmt.Sprintf("URL: %s", detailURL))

	// 使用 \r 作为换行符（微信格式）
	return strings.Join(parts, "\r")
}

// fetchFullBidInfo 查询完整的bid信息
func fetchFullBidInfo(id string) (*FullBidInfo, error) {
	query := `
		SELECT budget, site, buyer, industry, publish_time, bid_end_time
		FROM bids
		WHERE id = ?
	`
	var info FullBidInfo
	err := db.QueryRow(query, id).Scan(
		&info.Budget,
		&info.Site,
		&info.Buyer,
		&info.Industry,
		&info.PublishTime,
		&info.BidEndTime,
	)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// parseAIResponse 解析AI响应，提取JSON和思考过程
func parseAIResponse(responseText string, expectedCount int) (*BatchAnalysisResult, string) {
	var thinkingProcess string

	// 提取 <think> 标签中的内容
	thinkStart := strings.Index(responseText, "<think>")
	thinkEnd := strings.Index(responseText, "</think>")
	if thinkStart >= 0 && thinkEnd > thinkStart {
		thinkingProcess = strings.TrimSpace(responseText[thinkStart+7 : thinkEnd])
		responseText = strings.TrimSpace(responseText[thinkEnd+8:])
		log.Printf("[CRON] 提取到思考过程，长度: %d", len(thinkingProcess))
	}

	// 提取JSON部分
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		responseText = responseText[jsonStart : jsonEnd+1]
	}

	log.Printf("[CRON] 处理后JSON长度: %d", len(responseText))

	var result BatchAnalysisResult
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		log.Printf("[CRON] JSON解析失败: %v", err)
		log.Printf("[CRON] 问题响应片段: %s", responseText[:min(200, len(responseText))])

		// 解析失败时返回默认结果
		result.Projects = make([]ProjectAnalysisResult, expectedCount)
		for i := 0; i < expectedCount; i++ {
			result.Projects[i] = ProjectAnalysisResult{
				Index:          i + 1,
				Suitable:       false,
				Score:          -1,
				MatchLevel:     "需要人工审核",
				Priority:       "中",
				Reasons:        []string{"AI响应解析失败，建议人工审核"},
				Advantages:     []string{},
				Risks:          []string{"自动分析异常"},
				Recommendation: "请人工审核该项目",
			}
		}
		return &result, thinkingProcess
	}

	return &result, thinkingProcess
}
