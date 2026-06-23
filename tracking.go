// 追踪中标模块
// 包含追踪中标项目的添加、查询、删除、自动获取中标结果等功能
package main

import (
	"crypto/des"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"html"

	"github.com/gorilla/mux"
)

// ========== HTTP Handlers ==========

// trackWinnerHandler 添加项目到追踪列表
func trackWinnerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少项目ID"})
		return
	}
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS tracked_bids (
		id VARCHAR(255) PRIMARY KEY,
		serial VARCHAR(255),
		title TEXT NOT NULL,
		buyer TEXT,
		area TEXT,
		city TEXT,
		district TEXT,
		budget DOUBLE,
		bid_amount DOUBLE,
		buyer_person TEXT,
		buyer_tel TEXT,
		agency TEXT,
		agency_person TEXT,
		agency_tel TEXT,
		publish_time BIGINT,
		sign_end_time BIGINT,
		bid_end_time BIGINT,
		detail LONGTEXT,
		description LONGTEXT,
		industry TEXT,
		site TEXT,
		original_href TEXT,
		pdf_url TEXT,
		html_url TEXT,
		source VARCHAR(50),
		project_code TEXT,
		project_name TEXT,
		public_type TEXT,
		top_type TEXT,
		sub_type TEXT,
		purchasing TEXT,
		keywords TEXT,
		deliver_area TEXT,
		deliver_city TEXT,
		deliver_detail TEXT,
		china_bulletin_id TEXT,
		china_notice_media TEXT,
		china_bulletin_type TEXT,
		china_platform_name TEXT,
		china_region_code TEXT,
		china_bulletin_source TEXT,
		china_supervise_dept TEXT,
		china_data_source TEXT,
		china_classify_name TEXT,
		china_tender_agency TEXT,
		china_doc_get_end_time BIGINT,
		china_bid_doc_refer_end_time BIGINT,
		china_server_plat TEXT,
		china_trade_plat TEXT,
		china_data_plat TEXT,
		china_left_open_bid_day BIGINT,
		china_is_new INT,
		china_tender_project_name TEXT,
		china_bid_section_name TEXT,
		china_notice_name TEXT,
		china_potential_bidders LONGTEXT,
		china_similar_projects LONGTEXT,
		ai_score INT,
		ai_summary TEXT,
		ai_reason TEXT,
		ai_suggestion TEXT,
		ai_risk TEXT,
		ai_advantage TEXT,
		winner TEXT,
		winner_amount DOUBLE,
		win_time BIGINT,
		tracked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_check_time DATETIME NULL,
		winner_fetched TINYINT DEFAULT 0,
		winner_fetched_at DATETIME NULL,
		winner_detail LONGTEXT,
		winner_fetch_enabled TINYINT DEFAULT 0,
		winner_fetch_started_at DATETIME NULL,
		winner_fetch_attempts INT DEFAULT 0,
		winner_fetch_last_error TEXT
	)`
	if _, err := db.Exec(createTableSQL); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := ensureTrackedBidsEnhancements(); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var existing string
	if err := db.QueryRow("SELECT id FROM tracked_bids WHERE id = ?", req.ID).Scan(&existing); err == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"error_code": 0, "data": map[string]string{"message": "该项目已在跟踪列表中"}})
		return
	} else if err != sql.ErrNoRows {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var aiAnalysis sql.NullString
	if err := db.QueryRow("SELECT ai_analysis FROM bids WHERE id = ?", req.ID).Scan(&aiAnalysis); err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "未找到该项目信息"})
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var summary, reason, suggestion, risk, advantage string
	if aiAnalysis.Valid && aiAnalysis.String != "" {
		var analysis map[string]interface{}
		if err := json.Unmarshal([]byte(aiAnalysis.String), &analysis); err == nil {
			summary = pickString(analysis, "summary")
			reason = pickString(analysis, "reason")
			suggestion = pickString(analysis, "suggestion")
			risk = pickString(analysis, "risk")
			advantage = pickString(analysis, "advantage")
		}
	}
	_, err := db.Exec(`
		INSERT INTO tracked_bids (
			id, serial, title, buyer, area, city, district, budget, bid_amount,
			buyer_person, buyer_tel, agency, agency_person, agency_tel,
			publish_time, sign_end_time, bid_end_time,
			detail, description, industry, site, original_href, pdf_url, html_url,
			source, project_code, project_name, public_type, top_type, sub_type,
			purchasing, keywords, deliver_area, deliver_city, deliver_detail,
			china_bulletin_id, china_notice_media, china_bulletin_type, china_platform_name,
			china_region_code, china_bulletin_source, china_supervise_dept, china_data_source,
			china_classify_name, china_tender_agency, china_doc_get_end_time, china_bid_doc_refer_end_time,
			china_server_plat, china_trade_plat, china_data_plat, china_left_open_bid_day, china_is_new,
			china_tender_project_name, china_bid_section_name, china_notice_name,
			china_potential_bidders, china_similar_projects,
			ai_score, ai_summary, ai_reason, ai_suggestion, ai_risk, ai_advantage
		)
		SELECT 
			id, serial, title, buyer, area, city, district, budget, bid_amount,
			buyer_person, buyer_tel, agency, agency_person, agency_tel,
			publish_time, sign_end_time, bid_end_time,
			detail, description, industry, site, original_href, pdf_url, html_url,
			source, project_code, project_name, public_type, top_type, sub_type,
			purchasing, keywords, deliver_area, deliver_city, deliver_detail,
			china_bulletin_id, china_notice_media, china_bulletin_type, china_platform_name,
			china_region_code, china_bulletin_source, china_supervise_dept, china_data_source,
			china_classify_name, china_tender_agency, china_doc_get_end_time, china_bid_doc_refer_end_time,
			china_server_plat, china_trade_plat, china_data_plat, china_left_open_bid_day, china_is_new,
			china_tender_project_name, china_bid_section_name, china_notice_name,
			china_potential_bidders, china_similar_projects,
			ai_score, ?, ?, ?, ?, ?
		FROM bids WHERE id = ?
	`, summary, reason, suggestion, risk, advantage, req.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       map[string]string{"message": "已添加跟踪列表，将在投标截止之后持续查找中标候选人与中标通知"},
	})
}

// startTrackedWinnerFetch 开始获取追踪项目的中标结果
func startTrackedWinnerFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var body struct {
		WechatRoomID string `json:"wechat_room_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondError(w, http.StatusBadRequest, "缺少ID参数")
		return
	}
	var (
		source             sql.NullString
		chinaBulletinID    sql.NullString
		winnerFetched      sql.NullInt64
		winnerFetchEnabled sql.NullInt64
		trackCompleted     sql.NullInt64
		bidEndTime         sql.NullInt64
	)
	row := db.QueryRow(`SELECT source, china_bulletin_id, winner_fetched, winner_fetch_enabled, track_completed, bid_end_time FROM tracked_bids WHERE id = ?`, id)
	if err := row.Scan(&source, &chinaBulletinID, &winnerFetched, &winnerFetchEnabled, &trackCompleted, &bidEndTime); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "未找到跟踪记录")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if trackCompleted.Valid && trackCompleted.Int64 == 1 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "该项目已结束追踪，不可重新开始",
		})
		return
	}
	if winnerFetched.Valid && winnerFetched.Int64 == 1 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "该项目已获取中标结果",
		})
		return
	}
	sourceStr := strings.ToLower(source.String)
	if sourceStr != "china" && !chinaBulletinID.Valid {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error_code": 1,
			"error_msg":  "仅支持国央采或中国招标来源的自动获取",
		})
		return
	}
	nowUnix := time.Now().Unix()
	shouldDelay := false
	if bidEndTime.Valid && bidEndTime.Int64 > 0 && nowUnix < bidEndTime.Int64 {
		shouldDelay = true
	}

	if winnerFetchEnabled.Valid && winnerFetchEnabled.Int64 == 1 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"error_code": 0,
			"data": map[string]string{
				"message": func() string {
					if shouldDelay {
						return fmt.Sprintf("已开启自动追踪，将在投标截止（%s）后自动开始",
							time.Unix(bidEndTime.Int64, 0).Format("2006-01-02 15:04"))
					}
					return "已开始自动拉取，无需重复操作"
				}(),
			},
		})
		return
	}
	if body.WechatRoomID != "" {
		if _, err := db.Exec(`UPDATE tracked_bids SET wechat_room_id = ? WHERE id = ?`, body.WechatRoomID, id); err != nil {
			log.Printf("[追踪中标] 更新微信群ID失败: %v", err)
		}
	}

	if _, err := db.Exec(`UPDATE tracked_bids SET winner_fetch_enabled = 1, winner_fetch_started_at = NOW(), winner_fetch_attempts = 0, winner_fetch_last_error = NULL, last_check_time = NULL WHERE id = ?`, id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 立即尝试获取一次中标结果（异步执行，不阻塞响应）
	if !shouldDelay && chinaBulletinID.Valid && chinaBulletinID.String != "" {
		bulletinID := chinaBulletinID.String
		go func() {
			if err := fetchTrackedBidWinner(id, bulletinID); err != nil {
				log.Printf("[获取中标] 立即获取项目 %s 失败: %v", id, err)
			} else {
				log.Printf("[获取中标] 立即获取项目 %s 完成", id)
			}
		}()
	}

	message := "已启动自动拉取，正在立即获取中标信息，之后将每10分钟自动尝试"
	if shouldDelay {
		message = fmt.Sprintf("已开启自动追踪，将在投标截止（%s）后自动开始", time.Unix(bidEndTime.Int64, 0).Format("2006-01-02 15:04"))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]string{
			"message": message,
		},
	})
}

// stopTrackedWinnerFetch 停止获取追踪项目的中标结果
func stopTrackedWinnerFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondError(w, http.StatusBadRequest, "缺少ID参数")
		return
	}

	result, err := db.Exec(`UPDATE tracked_bids SET 
		winner_fetch_enabled = 0,
		winner_fetch_last_error = NULL,
		winner_fetch_started_at = NULL,
		last_check_time = NOW()
	WHERE id = ?`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "停止失败: "+err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondError(w, http.StatusNotFound, "记录不存在")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]string{
			"message": "已停止自动追踪",
		},
	})
}

// completeTrackedBid 结束追踪（标记该招标已完成追踪）
func completeTrackedBid(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondError(w, http.StatusBadRequest, "缺少ID参数")
		return
	}

	result, err := db.Exec(`UPDATE tracked_bids SET 
		winner_fetch_enabled = 0,
		track_completed = 1,
		track_completed_at = NOW()
	WHERE id = ?`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "结束追踪失败: "+err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondError(w, http.StatusNotFound, "记录不存在")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]string{
			"message": "已标记为结束追踪",
		},
	})
}

// getTrackedBids 获取追踪中标的招标列表
func getTrackedBids(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	search := r.URL.Query().Get("search")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// 构建查询条件
	whereClause := "1=1"
	args := []interface{}{}

	if search != "" {
		whereClause += " AND (title LIKE ? OR buyer LIKE ? OR serial LIKE ?)"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	// 查询总数
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tracked_bids WHERE %s", whereClause)
	err := db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}

	// 查询数据
	query := fmt.Sprintf(`SELECT id, serial, title, buyer, area, city, budget, industry, 
		publish_time, sign_end_time, bid_end_time, site, source, keywords, china_bulletin_id,
		ai_summary, ai_reason, ai_suggestion, ai_risk, ai_advantage, ai_score,
		last_check_time, winner, winner_amount, winner_fetched, winner_fetch_enabled,
		winner_fetch_started_at, winner_fetched_at, winner_fetch_attempts, winner_fetch_last_error,
		winner_detail, candidate_notified, candidate_notified_at, wechat_room_id,
		track_completed, track_completed_at, tracked_at 
		FROM tracked_bids WHERE %s ORDER BY tracked_at DESC LIMIT ? OFFSET ?`, whereClause)

	args = append(args, pageSize, offset)
	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	defer rows.Close()

	var bids []map[string]interface{}
	for rows.Next() {
		var (
			id                                   string
			serial                               sql.NullString
			title                                string
			buyer, area, city                    sql.NullString
			budget                               sql.NullFloat64
			industry, site, source, keywords     sql.NullString
			chinaBulletinID                      sql.NullString
			publishTime, signEndTime, bidEndTime sql.NullInt64
			aiSummary, aiReason, aiSuggestion    sql.NullString
			aiRisk, aiAdvantage                  sql.NullString
			aiScore                              sql.NullInt64
			lastCheckTime                        sql.NullTime
			winner                               sql.NullString
			winnerAmount                         sql.NullFloat64
			winnerFetched, winnerFetchEnabled    sql.NullInt64
			winnerFetchStartedAt                 sql.NullTime
			winnerFetchedAt                      sql.NullTime
			winnerFetchAttempts                  sql.NullInt64
			winnerFetchLastError                 sql.NullString
			winnerDetail                         sql.NullString
			candidateNotified                    sql.NullInt64
			candidateNotifiedAt                  sql.NullTime
			wechatRoomID                         sql.NullString
			trackCompleted                       sql.NullInt64
			trackCompletedAt                     sql.NullTime
			trackedAt                            sql.NullTime
		)

		err := rows.Scan(&id, &serial, &title, &buyer, &area, &city, &budget, &industry,
			&publishTime, &signEndTime, &bidEndTime, &site, &source, &keywords, &chinaBulletinID,
			&aiSummary, &aiReason, &aiSuggestion, &aiRisk, &aiAdvantage, &aiScore,
			&lastCheckTime, &winner, &winnerAmount, &winnerFetched, &winnerFetchEnabled,
			&winnerFetchStartedAt, &winnerFetchedAt, &winnerFetchAttempts, &winnerFetchLastError,
			&winnerDetail, &candidateNotified, &candidateNotifiedAt, &wechatRoomID,
			&trackCompleted, &trackCompletedAt, &trackedAt)
		if err != nil {
			log.Printf("[追踪中标] 扫描行失败: %v", err)
			continue
		}

		bid := map[string]interface{}{
			"id":    id,
			"title": title,
		}

		if serial.Valid {
			bid["serial"] = serial.String
		}
		if trackedAt.Valid {
			bid["created_at"] = trackedAt.Time.Format("2006-01-02 15:04:05")
		}

		if buyer.Valid {
			bid["buyer"] = buyer.String
		}
		if area.Valid {
			bid["area"] = area.String
		}
		if city.Valid {
			bid["city"] = city.String
		}
		if budget.Valid {
			bid["budget"] = budget.Float64
		}
		if industry.Valid {
			bid["industry"] = industry.String
		}
		if site.Valid {
			bid["site"] = site.String
		}
		if source.Valid {
			bid["source"] = source.String
		}
		if keywords.Valid {
			bid["keywords"] = keywords.String
		}
		if chinaBulletinID.Valid {
			bid["china_bulletin_id"] = chinaBulletinID.String
		}
		if publishTime.Valid {
			bid["publish_time"] = publishTime.Int64
		}
		if signEndTime.Valid {
			bid["sign_end_time"] = signEndTime.Int64
		}
		if bidEndTime.Valid {
			bid["bid_end_time"] = bidEndTime.Int64
		}
		if aiSummary.Valid {
			bid["ai_summary"] = aiSummary.String
		}
		if aiReason.Valid {
			bid["ai_reason"] = aiReason.String
		}
		if aiSuggestion.Valid {
			bid["ai_suggestion"] = aiSuggestion.String
		}
		if aiRisk.Valid {
			bid["ai_risk"] = aiRisk.String
		}
		if aiAdvantage.Valid {
			bid["ai_advantage"] = aiAdvantage.String
		}
		if aiScore.Valid {
			bid["ai_score"] = aiScore.Int64
		}
		if lastCheckTime.Valid {
			bid["last_check_time"] = lastCheckTime.Time.Format("2006-01-02 15:04:05")
		}
		if winner.Valid {
			bid["winner"] = winner.String
		}
		if winnerAmount.Valid {
			bid["winner_amount"] = winnerAmount.Float64
		}
		if winnerFetched.Valid {
			bid["winner_fetched"] = winnerFetched.Int64 == 1
		} else {
			bid["winner_fetched"] = false
		}
		if winnerFetchEnabled.Valid {
			bid["winner_fetch_enabled"] = winnerFetchEnabled.Int64 == 1
		} else {
			bid["winner_fetch_enabled"] = false
		}
		if winnerFetchStartedAt.Valid {
			bid["winner_fetch_started_at"] = winnerFetchStartedAt.Time.Format("2006-01-02 15:04:05")
		}
		if winnerFetchedAt.Valid {
			bid["winner_fetched_at"] = winnerFetchedAt.Time.Format("2006-01-02 15:04:05")
		}
		if winnerFetchAttempts.Valid {
			bid["winner_fetch_attempts"] = winnerFetchAttempts.Int64
		}
		if winnerFetchLastError.Valid {
			bid["winner_fetch_last_error"] = winnerFetchLastError.String
		}
		if candidateNotified.Valid {
			bid["candidate_notified"] = candidateNotified.Int64 == 1
		} else {
			bid["candidate_notified"] = false
		}
		if candidateNotifiedAt.Valid {
			bid["candidate_notified_at"] = candidateNotifiedAt.Time.Format("2006-01-02 15:04:05")
		}
		if wechatRoomID.Valid {
			bid["wechat_room_id"] = wechatRoomID.String
		}
		if trackCompleted.Valid {
			bid["track_completed"] = trackCompleted.Int64 == 1
		} else {
			bid["track_completed"] = false
		}
		if trackCompletedAt.Valid {
			bid["track_completed_at"] = trackCompletedAt.Time.Format("2006-01-02 15:04:05")
		}

		// 解析 winner_detail 提取详细信息
		if winnerDetail.Valid && winnerDetail.String != "" {
			bid["winner_detail_raw"] = winnerDetail.String
			if winnerInfo, err := parseWinnerInfo(winnerDetail.String); err == nil {
				infoMap := map[string]interface{}{
					"tender_notice_name":     winnerInfo.TenderNoticeName,
					"tender_notice_url":      winnerInfo.TenderNoticeURL,
					"candidate_notice_name":  winnerInfo.CandidateNoticeName,
					"candidate_notice_url":   winnerInfo.CandidateNoticeURL,
					"candidate_pdf_url":      winnerInfo.CandidatePDFURL,
					"candidate_publish_time": winnerInfo.CandidateNoticeTime,
					"result_notice_name":     winnerInfo.ResultNoticeName,
					"result_notice_url":      winnerInfo.ResultNoticeURL,
					"result_pdf_url":         winnerInfo.ResultPDFURL,
					"result_publish_time":    winnerInfo.ResultNoticeTime,
					"has_candidate":          winnerInfo.HasCandidate,
					"has_result":             winnerInfo.HasResult,
				}
				if len(winnerInfo.Candidates) > 0 {
					infoMap["candidate_list"] = winnerInfo.Candidates
				}
				bid["winner_info"] = infoMap
			}
		}

		bids = append(bids, bid)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"list":     bids,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// deleteTrackedBid 删除追踪中标记录
func deleteTrackedBid(w http.ResponseWriter, r *http.Request) {
	log.Printf("[删除追踪] 收到DELETE请求: %s", r.URL.Path)
	vars := mux.Vars(r)
	id := vars["id"]
	log.Printf("[删除追踪] 提取的ID: %s", id)

	if id == "" {
		respondError(w, http.StatusBadRequest, "缺少ID参数")
		return
	}

	result, err := db.Exec("DELETE FROM tracked_bids WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondError(w, http.StatusNotFound, "记录不存在")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "删除成功",
	})
}

// ========== 数据库辅助函数 ==========

// ensureTrackedBidsEnhancements 确保 tracked_bids 表有必要的字段
func ensureTrackedBidsEnhancements() error {
	alterStatements := []string{
		"ALTER TABLE tracked_bids ADD COLUMN winner_fetched_at DATETIME NULL",
		"ALTER TABLE tracked_bids ADD COLUMN winner_detail LONGTEXT",
		"ALTER TABLE tracked_bids ADD COLUMN winner_fetch_enabled TINYINT DEFAULT 0",
		"ALTER TABLE tracked_bids ADD COLUMN winner_fetch_started_at DATETIME NULL",
		"ALTER TABLE tracked_bids ADD COLUMN winner_fetch_attempts INT DEFAULT 0",
		"ALTER TABLE tracked_bids ADD COLUMN winner_fetch_last_error TEXT",
		"ALTER TABLE tracked_bids ADD COLUMN candidate_notified TINYINT DEFAULT 0",
		"ALTER TABLE tracked_bids ADD COLUMN candidate_notified_at DATETIME NULL",
		"ALTER TABLE tracked_bids ADD COLUMN wechat_room_id VARCHAR(255) NULL",
		"ALTER TABLE tracked_bids ADD COLUMN track_completed TINYINT DEFAULT 0",
		"ALTER TABLE tracked_bids ADD COLUMN track_completed_at DATETIME NULL",
	}
	for _, stmt := range alterStatements {
		if _, err := db.Exec(stmt); err != nil {
			lowerErr := strings.ToLower(err.Error())
			if strings.Contains(lowerErr, "duplicate column name") || strings.Contains(lowerErr, "doesn't exist") {
				continue
			}
			return err
		}
	}
	return nil
}

// ========== 中标信息解析 ==========

// parseWinnerInfo 解析中标信息
func parseWinnerInfo(detailsJSON string) (*WinnerInfo, error) {
	info := &WinnerInfo{
		FullDetails: detailsJSON,
	}
	if detailsJSON == "" {
		return nil, fmt.Errorf("详情为空")
	}

	// 首先检查 JSON 中是否包含"中标"关键词
	hasWinnerInfo := strings.Contains(detailsJSON, "中标") ||
		strings.Contains(detailsJSON, "候选人") ||
		strings.Contains(detailsJSON, "WinBidder") ||
		strings.Contains(detailsJSON, "winBidder")

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(detailsJSON), &result); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %v", err)
	}

	// 检查 success 字段
	if success, ok := result["success"].(bool); !ok || !success {
		return nil, fmt.Errorf("API返回失败")
	}

	// 获取 data 字段
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("data字段格式错误")
	}

	var winBidder string
	var winPrice interface{}

	for key, raw := range data {
		entry, ok := raw.(map[string]interface{})
		if !ok || entry == nil {
			continue
		}

		entryType := parseEntryType(entry, key)
		switch {
		case entryType == 1:
			applyTenderInfo(info, entry)
		case entryType == 2:
			applyCandidateInfo(info, entry, &winBidder, &winPrice)
		case entryType == 3:
			applyResultInfo(info, entry, &winBidder, &winPrice)
		default:
			if looksLikeCandidate(entry) {
				applyCandidateInfo(info, entry, &winBidder, &winPrice)
			} else if looksLikeResult(entry) {
				applyResultInfo(info, entry, &winBidder, &winPrice)
			} else if looksLikeTender(entry) {
				applyTenderInfo(info, entry)
			}
		}
	}

	// 如果仍然缺失招标公告信息，遍历一次尝试补全
	if info.TenderNoticeName == "" || info.TenderNoticeURL == "" {
		for _, raw := range data {
			entry, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if applyTenderInfo(info, entry) {
				break
			}
		}
	}

	info.Winner = winBidder

	// 如果 winBidder 为空但 JSON 中包含"中标"关键词，尝试从其他字段提取
	if winBidder == "" && hasWinnerInfo {
		for _, value := range data {
			if value == nil || value == "" {
				continue
			}
			if item, ok := value.(map[string]interface{}); ok {
				if bulletinJSONStr, ok := item["bulletinJSON"].(string); ok && bulletinJSONStr != "" {
					var bulletinJSON map[string]interface{}
					if err := json.Unmarshal([]byte(bulletinJSONStr), &bulletinJSON); err == nil {
						if val, ok := bulletinJSON["CandidateOrWinBidder"].(string); ok && val != "" {
							winBidder = val
						} else if val, ok := bulletinJSON["WinBidder"].(string); ok && val != "" {
							winBidder = val
						} else if val, ok := bulletinJSON["NOTICE_CONTENT"].(string); ok && val != "" {
							candidates := parseCandidatesFromContent(val)
							if len(candidates) > 0 {
								info.Candidates = candidates
								info.HasCandidate = true
								winBidder = candidates[0]
							}
						}
					}
				}
				if winBidder != "" {
					break
				}
			}
		}
	}

	// 根据是否存在结果公告，标记 HasResult
	if info.ResultNoticeName != "" || info.ResultNoticeURL != "" || info.ResultPDFURL != "" {
		if winBidder != "" {
			info.HasResult = true
		}
	}

	info.Winner = winBidder

	// 转换金额为 float64
	if winPrice != nil {
		switch v := winPrice.(type) {
		case float64:
			info.WinnerAmount = v
		case string:
			priceStr := strings.ReplaceAll(v, ",", "")
			priceStr = strings.ReplaceAll(priceStr, "元", "")
			priceStr = strings.TrimSpace(priceStr)
			if parsed, err := strconv.ParseFloat(priceStr, 64); err == nil {
				info.WinnerAmount = parsed
			}
		case int:
			info.WinnerAmount = float64(v)
		case int64:
			info.WinnerAmount = float64(v)
		}
	}

	return info, nil
}

func parseEntryType(entry map[string]interface{}, key string) int {
	if entry == nil {
		return 0
	}
	if val, ok := entry["type"]; ok {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				return parsed
			}
		}
	}
	if key != "" {
		if parsed, err := strconv.Atoi(key); err == nil {
			return parsed
		}
	}
	return 0
}

func applyTenderInfo(info *WinnerInfo, entry map[string]interface{}) bool {
	if entry == nil {
		return false
	}
	updated := false
	if val, ok := entry["bulletinName"].(string); ok && val != "" && info.TenderNoticeName == "" {
		info.TenderNoticeName = val
		updated = true
	}
	if val, ok := entry["noticeUrl"].(string); ok && val != "" && info.TenderNoticeURL == "" {
		info.TenderNoticeURL = val
		updated = true
	}
	if bulletinJSON := parseBulletinJSON(entry); bulletinJSON != nil {
		if val, ok := bulletinJSON["NOTICE_NAME"].(string); ok && val != "" && info.TenderNoticeName == "" {
			info.TenderNoticeName = val
			updated = true
		}
		if val, ok := bulletinJSON["NOTICE_URL"].(string); ok && val != "" && info.TenderNoticeURL == "" {
			info.TenderNoticeURL = val
			updated = true
		}
	}
	return updated
}

func applyCandidateInfo(info *WinnerInfo, entry map[string]interface{}, winBidder *string, winPrice *interface{}) {
	if entry == nil {
		return
	}
	if val, ok := entry["bulletinName"].(string); ok && val != "" && info.CandidateNoticeName == "" {
		info.CandidateNoticeName = val
	}
	if val, ok := entry["noticeUrl"].(string); ok && val != "" && info.CandidateNoticeURL == "" {
		info.CandidateNoticeURL = val
	}
	if val, ok := entry["noticeSendTime"].(string); ok && val != "" && info.CandidateNoticeTime == "" {
		info.CandidateNoticeTime = val
	}
	if val, ok := entry["pdfUrl"].(string); ok && val != "" {
		info.CandidatePDFURL = val
	}
	if val, ok := entry["winBidder"].(string); ok && val != "" && *winBidder == "" {
		*winBidder = val
	}
	if val, ok := entry["winPrice"]; ok && *winPrice == nil {
		*winPrice = val
	}
	if bulletinJSON := parseBulletinJSON(entry); bulletinJSON != nil {
		if val, ok := bulletinJSON["NOTICE_NAME"].(string); ok && val != "" && info.CandidateNoticeName == "" {
			info.CandidateNoticeName = val
		}
		if val, ok := bulletinJSON["NOTICE_CONTENT"].(string); ok && val != "" {
			if candidates := parseCandidatesFromContent(val); len(candidates) > 0 {
				info.Candidates = candidates
				info.HasCandidate = true
				if *winBidder == "" {
					*winBidder = candidates[0]
				}
			}
		}
		if val, ok := bulletinJSON["WinBidder"].(string); ok && val != "" && *winBidder == "" {
			*winBidder = val
		}
		if val, ok := bulletinJSON["WinPrice"]; ok && *winPrice == nil {
			*winPrice = val
		}
	}
}

func applyResultInfo(info *WinnerInfo, entry map[string]interface{}, winBidder *string, winPrice *interface{}) {
	if entry == nil {
		return
	}
	info.HasResult = true
	if val, ok := entry["bulletinName"].(string); ok && val != "" && info.ResultNoticeName == "" {
		info.ResultNoticeName = val
	}
	if val, ok := entry["noticeUrl"].(string); ok && val != "" && info.ResultNoticeURL == "" {
		info.ResultNoticeURL = val
	}
	if val, ok := entry["noticeSendTime"].(string); ok && val != "" && info.ResultNoticeTime == "" {
		info.ResultNoticeTime = val
	}
	if val, ok := entry["pdfUrl"].(string); ok && val != "" {
		info.ResultPDFURL = val
	}
	if val, ok := entry["winBidder"].(string); ok && val != "" {
		*winBidder = val
	}
	if val, ok := entry["winPrice"]; ok && *winPrice == nil {
		*winPrice = val
	}
	if bulletinJSON := parseBulletinJSON(entry); bulletinJSON != nil {
		if val, ok := bulletinJSON["NOTICE_NAME"].(string); ok && val != "" && info.ResultNoticeName == "" {
			info.ResultNoticeName = val
		}
		if val, ok := bulletinJSON["WinBidder"].(string); ok && val != "" {
			*winBidder = val
		}
		if val, ok := bulletinJSON["WinPrice"]; ok && *winPrice == nil {
			*winPrice = val
		}
	}
}

func parseBulletinJSON(entry map[string]interface{}) map[string]interface{} {
	if entry == nil {
		return nil
	}
	bulletinJSONStr, ok := entry["bulletinJSON"].(string)
	if !ok || bulletinJSONStr == "" {
		return nil
	}
	var bulletinJSON map[string]interface{}
	if err := json.Unmarshal([]byte(bulletinJSONStr), &bulletinJSON); err != nil {
		log.Printf("[解析公告JSON] 解析失败: %v", err)
		return nil
	}
	return bulletinJSON
}

func looksLikeCandidate(entry map[string]interface{}) bool {
	if entry == nil {
		return false
	}
	if val, ok := entry["bulletinName"].(string); ok && val != "" {
		if strings.Contains(val, "候选") || strings.Contains(val, "公示") {
			return true
		}
	}
	if val, ok := entry["type"]; ok {
		switch v := val.(type) {
		case string:
			return strings.Contains(v, "2")
		}
	}
	return false
}

func looksLikeResult(entry map[string]interface{}) bool {
	if entry == nil {
		return false
	}
	if val, ok := entry["bulletinName"].(string); ok && val != "" {
		if strings.Contains(val, "中标") && !strings.Contains(val, "候选") {
			return true
		}
	}
	if val, ok := entry["type"]; ok {
		switch v := val.(type) {
		case string:
			return strings.Contains(v, "3")
		}
	}
	return false
}

func looksLikeTender(entry map[string]interface{}) bool {
	if entry == nil {
		return false
	}
	if val, ok := entry["bulletinName"].(string); ok && val != "" {
		return strings.Contains(val, "招标") || strings.Contains(val, "公告")
	}
	return false
}

// parseCandidatesFromContent 从 NOTICE_CONTENT 中解析候选人信息
func parseCandidatesFromContent(content string) []string {
	if content == "" {
		return nil
	}

	// 1. 去掉 script/style/comment
	text := scriptTagPattern.ReplaceAllString(content, "")
	text = styleTagPattern.ReplaceAllString(text, "")
	text = htmlCommentPattern.ReplaceAllString(text, "")

	// 2. 将常见块级标签替换为换行
	text = newlineTagPattern.ReplaceAllString(text, "\n")

	// 3. 去掉剩余所有HTML标签
	text = htmlTagPattern.ReplaceAllString(text, "")

	// 4. 反转义 HTML 实体，并压缩空白
	text = html.UnescapeString(text)
	text = whitespacePattern.ReplaceAllString(text, " ")

	// 5. 使用 candidatePattern 提取“第X中标候选人：公司名”等结构
	matches := candidatePattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	namesMap := make(map[string]struct{})
	var names []string
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		name := strings.TrimSpace(m[2])
		if name == "" {
			continue
		}
		if _, exists := namesMap[name]; exists {
			continue
		}
		namesMap[name] = struct{}{}
		names = append(names, name)
	}

	return names
}

// ========== 获取中标结果 ==========

// fetchTrackedBidWinner 获取单个追踪项目的中标结果
func fetchTrackedBidWinner(id, chinaBulletinID string) error {
	if chinaBulletinID == "" {
		return fmt.Errorf("缺少 china_bulletin_id")
	}

	var candidateNotified bool
	var candidateFlag sql.NullInt64
	if err := db.QueryRow("SELECT candidate_notified FROM tracked_bids WHERE id = ?", id).Scan(&candidateFlag); err == nil {
		candidateNotified = candidateFlag.Valid && candidateFlag.Int64 == 1
	} else if err != sql.ErrNoRows {
		log.Printf("[获取中标] 查询candidate_notified失败: %v", err)
	}

	// 更新检查时间和尝试次数
	_, err := db.Exec(`UPDATE tracked_bids SET 
		last_check_time = NOW(), 
		winner_fetch_attempts = winner_fetch_attempts + 1 
	WHERE id = ?`, id)
	if err != nil {
		log.Printf("[获取中标] 更新检查时间失败: %v", err)
	}

	// 首先尝试从 tender_announcements 表中查询
	var winner sql.NullString
	var winPrice sql.NullString
	var detailsJSON sql.NullString
	var noticeTime sql.NullString

	query := `SELECT win_bidder, win_price, details, notice_time 
		FROM tender_announcements 
		WHERE bulletin_id = ? 
		ORDER BY notice_time DESC, id DESC 
		LIMIT 1`

	err = db.QueryRow(query, chinaBulletinID).Scan(&winner, &winPrice, &detailsJSON, &noticeTime)
	if err == nil && winner.Valid && winner.String != "" {
		// 从表中找到了记录
		winnerStr := winner.String

		// 转换金额
		var amount float64
		if winPrice.Valid && winPrice.String != "" {
			priceStr := strings.ReplaceAll(winPrice.String, ",", "")
			priceStr = strings.ReplaceAll(priceStr, "元", "")
			priceStr = strings.TrimSpace(priceStr)
			if parsed, parseErr := strconv.ParseFloat(priceStr, 64); parseErr == nil {
				amount = parsed
			}
		}

		detailsStr := ""
		if detailsJSON.Valid {
			detailsStr = detailsJSON.String
		}

		// 更新数据库
		_, err = db.Exec(`UPDATE tracked_bids SET 
			winner = ?,
			winner_amount = ?,
			winner_detail = ?,
			winner_fetched = 1,
			winner_fetched_at = NOW(),
			winner_fetch_last_error = NULL
		WHERE id = ?`, winnerStr, amount, detailsStr, id)
		if err != nil {
			return fmt.Errorf("更新数据库失败: %v", err)
		}
		log.Printf("[获取中标] 从数据库获取项目 %s 的中标信息: 中标单位=%s, 中标金额=%.2f", id, winnerStr, amount)

		// 如果详情JSON存在，尝试解析并发送微信通知
		if detailsStr != "" {
			if winnerInfo, parseErr := parseWinnerInfo(detailsStr); parseErr == nil && winnerInfo.Winner != "" {
				// 将PDF URL转换为本地文件路径
				if winnerInfo.CandidatePDFURL != "" {
					winnerInfo.CandidatePDFURL = getLocalPDFPath(chinaBulletinID, winnerInfo.CandidatePDFURL)
				}
				if winnerInfo.ResultPDFURL != "" {
					winnerInfo.ResultPDFURL = getLocalPDFPath(chinaBulletinID, winnerInfo.ResultPDFURL)
				}
				go sendWinnerWechatNotification(id, winnerInfo)
			}
		}
		return nil
	}

	// 如果表中没有找到，尝试通过API获取
	log.Printf("[获取中标] 表中未找到项目 %s (bulletinID: %s)，尝试通过API获取", id, chinaBulletinID)

	// 使用全局连接池 HTTP 客户端
	client := httpClient

	// 调用 fetchDetails 获取详情
	detailsStr, err := fetchDetails(client, chinaBulletinID)
	if err != nil {
		errorMsg := "获取详情失败: " + err.Error()
		db.Exec(`UPDATE tracked_bids SET winner_fetch_last_error = ? WHERE id = ?`, errorMsg, id)
		return fmt.Errorf("获取详情失败: %w", err)
	}

	// 解析中标信息
	winnerInfo, err := parseWinnerInfo(detailsStr)
	if err != nil {
		errorMsg := "解析中标信息失败: " + err.Error()
		db.Exec(`UPDATE tracked_bids SET winner_fetch_last_error = ? WHERE id = ?`, errorMsg, id)
		return fmt.Errorf("解析中标信息失败: %w", err)
	}

	// 下载PDF文件到本地
	if winnerInfo.CandidatePDFURL != "" {
		localPath, err := downloadPDFToLocal(chinaBulletinID, winnerInfo.CandidatePDFURL, "candidate")
		if err != nil {
			log.Printf("[获取中标] 下载候选人PDF失败: %v", err)
		} else if localPath != "" {
			winnerInfo.CandidatePDFURL = localPath
			// 立即更新原始JSON中的PDF URL
			detailsStr = updatePDFURLInJSON(detailsStr, "2", localPath)
		}
	}
	if winnerInfo.ResultPDFURL != "" {
		localPath, err := downloadPDFToLocal(chinaBulletinID, winnerInfo.ResultPDFURL, "result")
		if err != nil {
			log.Printf("[获取中标] 下载结果PDF失败: %v", err)
		} else if localPath != "" {
			winnerInfo.ResultPDFURL = localPath
			// 立即更新原始JSON中的PDF URL
			detailsStr = updatePDFURLInJSON(detailsStr, "3", localPath)
		}
	}

	// 先更新原始详情JSON，便于前端展示候选人/结果公告等完整信息
	_, err = db.Exec(`UPDATE tracked_bids SET 
		winner_detail = ?,
		winner_fetch_last_error = NULL
	WHERE id = ?`, detailsStr, id)
	if err != nil {
		log.Printf("[获取中标] 更新winner_detail失败: %v", err)
	}

	// 如果既没有候选人也没有中标结果，可能还未发布任何公告
	if !winnerInfo.HasCandidate && !winnerInfo.HasResult && winnerInfo.Winner == "" {
		log.Printf("[获取中标] 项目 %s (bulletinID: %s) 尚未发布候选人/中标信息", id, chinaBulletinID)
		return nil
	}

	// 更新展示用的 winner / winner_amount 字段：
	// - 如果有最终结果（HasResult），使用最终中标人和金额
	// - 如果只有候选人（HasCandidate），使用第一候选人名称，但不标记 winner_fetched，保持持续拉取
	if winnerInfo.Winner != "" {
		_, err = db.Exec(`UPDATE tracked_bids SET 
			winner = ?,
			winner_amount = ?
		WHERE id = ?`, winnerInfo.Winner, winnerInfo.WinnerAmount, id)
		if err != nil {
			log.Printf("[获取中标] 更新winner字段失败: %v", err)
		}
	}

	// 是否需要发送最终中标结果通知（只要有结果公告就通知）
	shouldSendResult := winnerInfo.HasResult
	// 是否可以认为已经拿到最终中标人（有结果公告且解析到了 Winner）
	resultFetched := winnerInfo.HasResult && winnerInfo.Winner != ""
	shouldSendCandidate := winnerInfo.HasCandidate && !winnerInfo.HasResult && !candidateNotified

	if resultFetched {
		_, err = db.Exec(`UPDATE tracked_bids SET 
			winner_fetched = 1,
			winner_fetched_at = NOW(),
			candidate_notified = 1
		WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("更新最终中标状态失败: %v", err)
		}
		log.Printf("[获取中标] 通过API成功获取项目 %s 的中标信息: 中标单位=%s, 中标金额=%.2f", id, winnerInfo.Winner, winnerInfo.WinnerAmount)
	} else if winnerInfo.HasResult {
		// 有结果公告但暂未解析到明确的中标单位，仍视为结果阶段，仅不标记 winner_fetched=1
		log.Printf("[获取中标] 项目 %s 获取到中标结果公告（未解析到中标单位）", id)
	} else if winnerInfo.HasCandidate {
		log.Printf("[获取中标] 项目 %s 获取到中标候选人信息（尚无最终结果）: %s", id, strings.Join(winnerInfo.Candidates, ", "))
	}

	if shouldSendCandidate {
		if _, err := db.Exec(`UPDATE tracked_bids SET candidate_notified = 1 WHERE id = ?`, id); err != nil {
			log.Printf("[获取中标] 更新candidate_notified失败: %v", err)
		} else {
			candidateNotified = true
		}
	}

	// 发送微信通知：
	// - 候选阶段：推送候选人公示（每条仅一次）
	// - 结果阶段：推送最终中标结果
	if shouldSendResult || shouldSendCandidate {
		go sendWinnerWechatNotification(id, winnerInfo)
	}

	return nil
}

// fetchTrackedBidsWinners 批量获取追踪项目的中标结果
func fetchTrackedBidsWinners() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[获取中标] 定时任务发生panic: %v", r)
		}
	}()

	now := time.Now()
	hour := now.Hour()
	if hour >= 23 || hour < 7 {
		log.Printf("[获取中标] 当前时间 %s 属于休眠时段(23:00-07:00)，暂停自动抓取", now.Format("2006-01-02 15:04:05"))
		return
	}

	// 查询需要获取中标结果的项目
	query := `SELECT id, china_bulletin_id 
		FROM tracked_bids 
		WHERE winner_fetch_enabled = 1 
		AND winner_fetched = 0 
		AND china_bulletin_id IS NOT NULL 
		AND china_bulletin_id != ''
		AND (bid_end_time IS NULL OR bid_end_time = 0 OR bid_end_time <= UNIX_TIMESTAMP())
		ORDER BY winner_fetch_started_at ASC
		LIMIT 10`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("[获取中标] 查询失败: %v", err)
		return
	}
	defer rows.Close()

	var tasks []struct {
		ID              string
		ChinaBulletinID string
	}

	for rows.Next() {
		var task struct {
			ID              string
			ChinaBulletinID sql.NullString
		}
		if err := rows.Scan(&task.ID, &task.ChinaBulletinID); err != nil {
			log.Printf("[获取中标] 扫描数据失败: %v", err)
			continue
		}
		if task.ChinaBulletinID.Valid {
			tasks = append(tasks, struct {
				ID              string
				ChinaBulletinID string
			}{
				ID:              task.ID,
				ChinaBulletinID: task.ChinaBulletinID.String,
			})
		}
	}

	if len(tasks) == 0 {
		return
	}

	log.Printf("[获取中标] 找到 %d 个需要获取中标结果的项目", len(tasks))

	// 逐个获取中标结果
	for _, task := range tasks {
		if err := fetchTrackedBidWinner(task.ID, task.ChinaBulletinID); err != nil {
			log.Printf("[获取中标] 项目 %s 获取失败: %v", task.ID, err)
		}
		// 避免请求过快
		time.Sleep(500 * time.Millisecond)
	}
}

// ========== PDF处理 ==========

// downloadPDFToLocal 下载PDF文件到本地存储
func downloadPDFToLocal(bulletinID, pdfUrl, pdfType string) (string, error) {
	if bulletinID == "" || pdfUrl == "" {
		return "", fmt.Errorf("缺少必要参数")
	}

	// 构建完整的PDF URL
	fullURL := pdfUrl
	if !strings.HasPrefix(pdfUrl, "http://") && !strings.HasPrefix(pdfUrl, "https://") {
		fullURL = "https://bigdata.cebpubservice.com/" + strings.TrimPrefix(pdfUrl, "/")
	}

	// 获取存储目录
	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "storage/r2"
	}
	chinaDir := filepath.Join(storageDir, "china")
	if err := os.MkdirAll(chinaDir, 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成文件名：{bulletinId}-{type}-{timestamp}.pdf
	fileName := fmt.Sprintf("%s-%s-%d.pdf", bulletinID, pdfType, time.Now().UnixMilli())
	localPath := filepath.Join(chinaDir, fileName)

	// 下载PDF文件 — 使用全局长超时连接池客户端
	client := longHTTPClient
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 保存文件
	tmpPath := localPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	if written == 0 {
		os.Remove(tmpPath)
		return "", fmt.Errorf("文件为空")
	}

	// 重命名为最终文件名
	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("重命名文件失败: %w", err)
	}

	// 返回相对路径
	relativePath := filepath.Join("china", fileName)
	log.Printf("[下载PDF] 成功下载PDF: %s -> %s", fullURL, relativePath)
	return relativePath, nil
}

// updatePDFURLInJSON 更新JSON字符串中指定data key的pdfUrl字段
func updatePDFURLInJSON(jsonStr, dataKey, newPDFURL string) string {
	if jsonStr == "" || newPDFURL == "" {
		return jsonStr
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("[更新PDF URL] 解析JSON失败: %v", err)
		return jsonStr
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return jsonStr
	}

	// 更新 data[dataKey] 中的 pdfUrl
	if item, ok := data[dataKey].(map[string]interface{}); ok {
		item["pdfUrl"] = newPDFURL
		log.Printf("[更新PDF URL] 更新 data[%s].pdfUrl = %s", dataKey, newPDFURL)
	}

	// 重新序列化
	updatedJSON, err := json.Marshal(result)
	if err != nil {
		log.Printf("[更新PDF URL] 序列化JSON失败: %v", err)
		return jsonStr
	}

	return string(updatedJSON)
}

// getLocalPDFPath 根据 bulletinId 查找本地PDF文件
func getLocalPDFPath(bulletinID, pdfUrl string) string {
	if bulletinID == "" {
		return ""
	}

	// 获取存储目录
	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "storage/r2"
	}
	chinaDir := filepath.Join(storageDir, "china")

	// 尝试查找匹配的PDF文件：{bulletinId}-*.pdf
	pattern := filepath.Join(chinaDir, bulletinID+"-*.pdf")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	// 使用第一个匹配的文件（通常是最新的）
	matchedFile := matches[0]
	filename := filepath.Base(matchedFile)

	return filename
}

// ========== API调用 ==========

// fetchDetails 从API获取项目详情
func fetchDetails(client *http.Client, bulletinID string) (string, error) {
	url := fmt.Sprintf("https://ctbpsp.com/cutominfoapi/selectRelevantBulletin?bulletinId=%s", bulletinID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Host", "ctbpsp.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	bodyStr := strings.TrimSpace(string(bodyBytes))
	if len(bodyStr) > 1 && strings.HasPrefix(bodyStr, "\"") && strings.HasSuffix(bodyStr, "\"") {
		bodyStr = bodyStr[1 : len(bodyStr)-1]
	}

	if strings.HasPrefix(bodyStr, "{") {
		return bodyStr, nil
	}

	decryptedBytes, err := decryptByDES(bodyStr)
	if err != nil {
		snippet := bodyStr
		if len(snippet) > 100 {
			snippet = snippet[:100] + "..."
		}
		return "", fmt.Errorf("decryption failed: %v. Body snippet: %s", err, snippet)
	}

	return string(decryptedBytes), nil
}

// decryptByDES 使用DES算法解密Base64编码的密文
func decryptByDES(ciphertext string) ([]byte, error) {
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		ciphertextBytes, err = base64.RawURLEncoding.DecodeString(ciphertext)
		if err != nil {
			return nil, err
		}
	}

	keyBytes := []byte("1qaz@wsx3e")
	if len(keyBytes) > 8 {
		keyBytes = keyBytes[:8]
	}
	block, err := des.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}

	if len(ciphertextBytes)%des.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}

	plaintextBytes := make([]byte, len(ciphertextBytes))
	for i := 0; i < len(ciphertextBytes); i += des.BlockSize {
		block.Decrypt(plaintextBytes[i:i+des.BlockSize], ciphertextBytes[i:i+des.BlockSize])
	}

	return unpad(plaintextBytes, des.BlockSize)
}

// unpad 移除DES解密后的填充数据
func unpad(src []byte, blockSize int) ([]byte, error) {
	length := len(src)
	if length == 0 {
		return nil, fmt.Errorf("source data is empty")
	}
	padding := int(src[length-1])
	if padding > blockSize || padding > length {
		return nil, fmt.Errorf("invalid padding")
	}
	return src[:length-padding], nil
}
