// 公司中标历史查询相关功能
// 提供实时搜索公司中标记录、获取详情、DES解密等功能
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func runCompanyAutoFetch() {
	companyAutoFetchState.mu.Lock()
	if companyAutoFetchState.running {
		companyAutoFetchState.mu.Unlock()
		log.Println("[企业自动获取] 已有任务在运行，跳过本次请求")
		return
	}
	companyAutoFetchState.running = true
	companyAutoFetchState.startedAt = time.Now()
	companyAutoFetchState.finishedAt = time.Time{}
	companyAutoFetchState.totalCount = 0
	companyAutoFetchState.successCount = 0
	companyAutoFetchState.failedCount = 0
	companyAutoFetchState.lastMessage = "任务启动中"
	companyAutoFetchState.mu.Unlock()

	defer func() {
		companyAutoFetchState.mu.Lock()
		companyAutoFetchState.running = false
		companyAutoFetchState.finishedAt = time.Now()
		companyAutoFetchState.lastMessage = fmt.Sprintf("更新完成，成功 %d 家，失败 %d 家", companyAutoFetchState.successCount, companyAutoFetchState.failedCount)
		companyAutoFetchState.mu.Unlock()
	}()

	rows, err := db.Query("SELECT DISTINCT search_company FROM tender_announcements WHERE search_company IS NOT NULL AND search_company <> ''")
	if err != nil {
		log.Printf("[企业自动获取] 查询企业列表失败: %v", err)
		return
	}
	defer rows.Close()

	var companies []string
	for rows.Next() {
		var company sql.NullString
		if err := rows.Scan(&company); err != nil {
			continue
		}
		if company.Valid && strings.TrimSpace(company.String) != "" {
			companies = append(companies, strings.TrimSpace(company.String))
		}
	}
	companyAutoFetchState.mu.Lock()
	companyAutoFetchState.totalCount = len(companies)
	companyAutoFetchState.mu.Unlock()

	for _, company := range companies {
		payload, _ := json.Marshal(map[string]string{"company": company})
		req, _ := http.NewRequest(http.MethodPost, "/internal/company-awards/search-realtime", bytes.NewBuffer(payload))
		recorder := &responseBuffer{header: http.Header{}}
		searchCompanyRealtime(recorder, req)
		companyAutoFetchState.mu.Lock()
		if recorder.status >= 400 {
			companyAutoFetchState.failedCount++
		} else {
			companyAutoFetchState.successCount++
		}
		companyAutoFetchState.mu.Unlock()
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("[企业自动获取] 已完成 %d 家企业更新", len(companies))
}

func getCompanyAutoFetchStatus(w http.ResponseWriter, r *http.Request) {
	companyAutoFetchState.mu.Lock()
	running := companyAutoFetchState.running
	startedAt := companyAutoFetchState.startedAt
	finishedAt := companyAutoFetchState.finishedAt
	totalCount := companyAutoFetchState.totalCount
	successCount := companyAutoFetchState.successCount
	failedCount := companyAutoFetchState.failedCount
	lastMessage := companyAutoFetchState.lastMessage
	companyAutoFetchState.mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"running":       running,
			"started_at":    startedAt,
			"finished_at":   finishedAt,
			"total_count":   totalCount,
			"success_count": successCount,
			"failed_count":  failedCount,
			"last_message":  lastMessage,
		},
	})
}

func runCompanyAutoFetchHandler(w http.ResponseWriter, r *http.Request) {
	companyAutoFetchState.mu.Lock()
	if companyAutoFetchState.running {
		companyAutoFetchState.mu.Unlock()
		respondJSON(w, http.StatusConflict, map[string]interface{}{"error": "企业自动获取任务正在运行中"})
		return
	}
	companyAutoFetchState.mu.Unlock()

	go runCompanyAutoFetch()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "企业自动获取任务已启动",
	})
}

type responseBuffer struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *responseBuffer) Header() http.Header            { return r.header }
func (r *responseBuffer) Write(data []byte) (int, error) { return r.body.Write(data) }
func (r *responseBuffer) WriteHeader(statusCode int)     { r.status = statusCode }

// ExternalRequestBody 外部API请求体结构
// 用于向中国招标投标公共服务平台发送查询请求
type ExternalRequestBody struct {
	KeyWord                string `json:"keyWord"`
	RegionName             string `json:"regionName"`
	BidSectionClassifyCode string `json:"bidSectionClassifyCode"`
	BeginTime              string `json:"beginTime"`
	EndTime                string `json:"endTime"`
	CurrentPage            int    `json:"currentPage"`
	PageSize               int    `json:"pageSize"`
	BulletinType           string `json:"bulletinType"`
	AgencyType             string `json:"agencyType"`
}

// ExternalResponse 外部API响应结构
// 用于解析中国招标投标公共服务平台返回的响应数据
type ExternalResponse struct {
	Code    int  `json:"code"`
	Success bool `json:"success"`
	Data    struct {
		Success bool `json:"success"`
		Data    struct {
			DataList []struct {
				BulletinID        string `json:"bulletinID"`
				TenderProjectName string `json:"tenderProjectName"`
				WinBidder         string `json:"winBidder"`
				WinPrice          string `json:"winPrice"`
				NoticeSendTime    string `json:"noticeSendTime"`
				NoticeUrl         string `json:"noticeUrl"`
			} `json:"dataList"`
			TotalPage   int `json:"totalPage"`
			CurrentPage int `json:"currentPage"`
			TotalCount  int `json:"totalCount"`
			PageSize    int `json:"pageSize"`
		} `json:"data"`
	} `json:"data"`
	Msg string `json:"msg"`
}

// searchCompanyRealtime 实时搜索公司中标历史
// 从中国招标投标公共服务平台获取指定公司的中标记录
// 支持分页查询，自动获取详情并保存到数据库
func searchCompanyRealtime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Company string `json:"company"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "Invalid JSON"})
		return
	}

	company := strings.TrimSpace(req.Company)
	if company == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "Company name is required"})
		return
	}

	// Create table if not exists (ensure schema matches user's description)
	// We assume the table exists as per user statement, but we can try to ensure it.
	// For now, we skip table creation to avoid altering user's manual changes incorrectly.

	client := httpClient

	currentPage := 1
	totalPage := 1 // Start with 1, update after first request
	maxPages := 5  // Limit to 5 pages for real-time search to be fast, or let it run?
	// User said "Real-time search", usually implies speed. But "Find history" implies completeness.
	// Let's fetch up to 10 pages.
	maxPages = 10

	var allItems []map[string]interface{}

	for currentPage <= totalPage && currentPage <= maxPages {
		// Construct URL with query parameter
		baseURL := "https://bulletin.cebpubservice.com/agency/api/agency-business/homepage-block/tender-announcement"
		params := url.Values{}
		params.Add("tenantName", company)
		fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

		// Construct Request Body
		reqBody := ExternalRequestBody{
			KeyWord:     "",
			RegionName:  "",
			CurrentPage: currentPage,
			PageSize:    20, // Smaller page size for faster initial response? User used 300.
			// If we use 300, we might get everything in 1 request.
			// Let's use 100.
			BulletinType: "5",
			AgencyType:   "30",
		}
		// User used PageSize 300. Let's stick to a reasonable number.
		reqBody.PageSize = 50

		jsonData, _ := json.Marshal(reqBody)

		// Create Request
		req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Error creating request: %v", err)
			break
		}

		// Set Headers
		req.Header.Set("Host", "bulletin.cebpubservice.com")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://bulletin.cebpubservice.com")
		req.Header.Set("sec-ch-ua-platform", "\"Windows\"")

		// Send Request
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error sending request for %s page %d: %v", company, currentPage, err)
			break
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var apiResp ExternalResponse
		if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
			log.Printf("Error parsing JSON for %s: %v", company, err)
			break
		}

		if !apiResp.Success || !apiResp.Data.Success {
			log.Printf("API returned error for %s: %s", company, apiResp.Msg)
			break
		}

		// Update totalPage from the first response
		if currentPage == 1 {
			totalPage = apiResp.Data.Data.TotalPage
			if totalPage == 0 {
				break
			}
		}

		// Insert data into database
		for _, item := range apiResp.Data.Data.DataList {
			err := insertTender(db, company, item.BulletinID, item.TenderProjectName, item.WinBidder, item.WinPrice, item.NoticeSendTime, item.NoticeUrl)
			if err != nil {
				log.Printf("Error inserting data for %s: %v", item.TenderProjectName, err)
			} else {
				if details, derr := fetchDetails(client, item.BulletinID); derr != nil {
					log.Printf("Error fetching details for %s: %v", item.BulletinID, derr)
				} else if derr = updateTenderDetails(db, company, item.BulletinID, details); derr != nil {
					log.Printf("Error updating details for %s: %v", item.BulletinID, derr)
				}
				time.Sleep(200 * time.Millisecond)
			}

			allItems = append(allItems, map[string]interface{}{
				"bulletin_id":  item.BulletinID,
				"project_name": item.TenderProjectName,
				"win_bidder":   item.WinBidder,
				"win_price":    item.WinPrice,
				"notice_time":  item.NoticeSendTime,
				"notice_url":   item.NoticeUrl,
			})
		}

		currentPage++
		time.Sleep(200 * time.Millisecond)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       allItems,
		"message":    fmt.Sprintf("Fetched %d items", len(allItems)),
	})
}

// insertTender 插入或更新招标公告记录到数据库
// 如果记录已存在（基于 bulletin_id），则更新相关信息
// 参数:
//   - db: 数据库连接对象
//   - company: 搜索的公司名称
//   - bulletinID: 公告ID
//   - projectName: 项目名称
//   - winBidder: 中标单位
//   - winPrice: 中标金额
//   - noticeTime: 公告时间
//   - noticeUrl: 公告URL
func insertTender(db *sql.DB, company, bulletinID, projectName, winBidder, winPrice, noticeTime, noticeUrl string) error {
	// Check if 'details' column exists, if so we might want to update it?
	// For now, we just insert the basic info.
	// The user said they added 'details', so we should be careful not to break if we don't provide it.
	// But INSERT INTO ... (cols) VALUES ... works fine if we don't specify nullable columns.

	query := `
	INSERT INTO tender_announcements (search_company, bulletin_id, project_name, win_bidder, win_price, notice_time, notice_url)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		project_name = VALUES(project_name),
		win_bidder = VALUES(win_bidder),
		win_price = VALUES(win_price),
		notice_time = VALUES(notice_time),
		notice_url = VALUES(notice_url);
	`
	_, err := db.Exec(query, company, bulletinID, projectName, winBidder, winPrice, noticeTime, noticeUrl)
	return err
}

// updateTenderDetails 更新招标公告的详细信息
// 参数:
//   - db: 数据库连接对象
//   - company: 搜索的公司名称
//   - bulletinID: 公告ID
//   - details: 详细信息（JSON格式）
func updateTenderDetails(db *sql.DB, company, bulletinID, details string) error {
	query := `
	UPDATE tender_announcements
	SET details = ?
	WHERE search_company = ? AND bulletin_id = ?
	`
	_, err := db.Exec(query, details, company, bulletinID)
	return err
}
