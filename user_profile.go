// 用户画像 + 每日问候系统
// 记录用户 IP/设备/访问习惯，AI 预生成每日问候语和要闻
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ========== 数据库表初始化 ==========

func ensureUserProfileTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS daily_content (
			id INT AUTO_INCREMENT PRIMARY KEY,
			date DATE NOT NULL UNIQUE COMMENT '日期',
			greeting VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'AI 生成的问候语',
			quote TEXT COMMENT 'AI 生成的今日语录',
			news_summary TEXT COMMENT 'AI 生成的今日招标要闻',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_date (date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS user_profiles (
			id INT AUTO_INCREMENT PRIMARY KEY,
			ip_hash VARCHAR(64) NOT NULL COMMENT 'IP 哈希（隐私保护）',
			user_agent TEXT COMMENT '用户代理',
			device_type VARCHAR(20) DEFAULT 'desktop' COMMENT '设备类型: desktop/mobile/tablet',
			platform VARCHAR(50) DEFAULT '' COMMENT '操作系统平台',
			today_visited DATE DEFAULT NULL COMMENT '今日首次访问日期',
			last_visit_date DATE DEFAULT NULL COMMENT '最后访问日期',
			visit_count INT DEFAULT 0 COMMENT '累计访问次数',
			first_visit_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_ip (ip_hash),
			INDEX idx_today (today_visited)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for _, sql := range tables {
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}
	return nil
}

// ========== 设备识别 ==========

type DeviceInfo struct {
	DeviceType string `json:"device_type"` // desktop / mobile / tablet
	Platform   string `json:"platform"`    // Windows / macOS / iOS / Android / Linux
}

func detectDevice(userAgent string) DeviceInfo {
	ua := strings.ToLower(userAgent)
	info := DeviceInfo{DeviceType: "desktop", Platform: "Unknown"}

	if strings.Contains(ua, "windows") {
		info.Platform = "Windows"
	} else if strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os") {
		info.Platform = "macOS"
	} else if strings.Contains(ua, "linux") && !strings.Contains(ua, "android") {
		info.Platform = "Linux"
	} else if strings.Contains(ua, "android") {
		info.Platform = "Android"
		info.DeviceType = "mobile"
	} else if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		if strings.Contains(ua, "ipad") {
			info.DeviceType = "tablet"
		} else {
			info.DeviceType = "mobile"
		}
		info.Platform = "iOS"
	}

	// Tablet 覆盖检测
	if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		info.DeviceType = "tablet"
	}

	return info
}

func hashIP(ip string) string {
	h := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(h[:])
}

// ========== AI 每日内容生成 ==========

// generateDailyContent 通过 CTYun AI 预生成今日问候语 + 语录 + 要闻
func generateDailyContent() (greeting, quote, newsSummary string) {
	today := time.Now().Format("2006年1月2日")
	weekday := time.Now().Weekday().String()
	weekdayCN := map[string]string{
		"Monday": "星期一", "Tuesday": "星期二", "Wednesday": "星期三",
		"Thursday": "星期四", "Friday": "星期五", "Saturday": "星期六", "Sunday": "星期日",
	}[weekday]

	// 生成问候语
	greeting = generateAIText(fmt.Sprintf(
		`你是一个可爱的招标助手「小招AI」。今天是 %s %s。
请用一句温暖的话向用户打招呼，包含日期和星期，风格要商务又可爱，可以带 emoji，不超过 30 个字。
例如："%s 早上好！今天也是元气满满的一天 ✦"`,
		today, weekdayCN, today))

	// 生成语录
	quote = generateAIText(`请用一句话写一个鼓励投标人或职场人的励志语录，
风格温暖可爱，可以带 emoji，不超过 25 个字。
不要加引号和前缀。例如："每一次投标都是一次成长，加油 💪"`)

	// 生成今日招标要闻摘要
	newsSummary = generateAIText(fmt.Sprintf(
		`今天是 %s %s。请以招标助手的身份，用 2-3 句话写一段今日招标市场动态摘要，
风格专业又亲切，提及今日是周几，
可以带 emoji，不超过 80 个字。`,
		today, weekdayCN))

	return
}

// generateAIText 调用 CTYun 生成一段简短文本（非流式）
func generateAIText(prompt string) string {
	if ctyunClient == nil {
		return ""
	}

	if err := ctyunClient.EnsureSession(); err != nil {
		log.Printf("[每日内容] CTYun 会话失效: %v", err)
		return ""
	}

	model := getConfigValue("CTYUN_MODEL", "TEXT_DEEPSEEK_V4")
	xuid := "pubweb_" + ctyRandomHex(16)

	body := map[string]interface{}{
		"key_model": model,
		"messages": []map[string]interface{}{
			{
				"role":      "user",
				"content":   prompt,
				"verify_id": ctyRandomHex(16),
				"ref": map[string]interface{}{
					"type": "file",
					"file": []CTYunUploadedFile{},
				},
			},
		},
		"stream":          true,
		"client_retry":    true,
		"web_search":      true,
		"tenantId":        15,
		"enable_thinking": false,
	}

	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", ctyChatURL, bytes.NewReader(bodyBytes))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "*/*")
	hdr := ctyMakeEAIHeaders(xuid)
	for k, vals := range hdr {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	resp, err := ctyunClient.httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		log.Printf("[每日内容] API 请求失败: %v", err)
		return ""
	}
	defer resp.Body.Close()

	// 读取流式响应
	var result strings.Builder
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(line[5:])
		if payload == "[DONE]" {
			continue
		}
		var evt struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}
		for _, ch := range evt.Choices {
			result.WriteString(ch.Delta.Content)
		}
	}

	return strings.TrimSpace(result.String())
}

// ========== API 处理器 ==========

// mascotDailyHandler GET /api/mascot/daily
// 用户每日首次访问时调用，返回预加载的问候内容
func mascotDailyHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 解析请求
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = strings.Split(forwarded, ",")[0]
	}
	userAgent := r.Header.Get("User-Agent")
	device := detectDevice(userAgent)
	ipHashed := hashIP(ip)
	today := time.Now().Format("2006-01-02")

	// 2. 更新用户画像
	isFirstToday := false
	var existingToday sql.NullString
	err := db.QueryRow("SELECT today_visited FROM user_profiles WHERE ip_hash = ?", ipHashed).Scan(&existingToday)
	if err == sql.ErrNoRows {
		// 新用户
		_, err = db.Exec(
			"INSERT INTO user_profiles (ip_hash, user_agent, device_type, platform, today_visited, last_visit_date, visit_count) VALUES (?, ?, ?, ?, ?, ?, 1)",
			ipHashed, userAgent, device.DeviceType, device.Platform, today, today,
		)
		isFirstToday = true
	} else if err == nil {
		// 老用户
		if existingToday.Valid && existingToday.String == today {
			isFirstToday = false
		} else {
			isFirstToday = true
		}
		_, _ = db.Exec(
			"UPDATE user_profiles SET today_visited = ?, last_visit_date = ?, visit_count = visit_count + 1, device_type = ?, platform = ?, user_agent = ? WHERE ip_hash = ?",
			today, today, device.DeviceType, device.Platform, userAgent, ipHashed,
		)
	}

	// 3. 获取/生成今日内容
	greeting := ""
	quote := ""
	newsSummary := ""

	err = db.QueryRow("SELECT greeting, quote, news_summary FROM daily_content WHERE date = ?", today).Scan(&greeting, &quote, &newsSummary)
	if err == sql.ErrNoRows {
		// 今日内容未生成 → 调用 AI 生成
		log.Printf("[每日内容] %s 首次请求，正在通过 AI 预加载...", today)
		greeting, quote, newsSummary = generateDailyContent()
		if greeting != "" || quote != "" {
			_, _ = db.Exec(
				"INSERT INTO daily_content (date, greeting, quote, news_summary) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE greeting=VALUES(greeting), quote=VALUES(quote), news_summary=VALUES(news_summary)",
				today, greeting, quote, newsSummary,
			)
			log.Printf("[每日内容] %s AI 内容已生成并存入数据库", today)
		}
	}

	// 4. 返回
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"is_first_today": isFirstToday,
		"date":           today,
		"greeting":       greeting,
		"quote":          quote,
		"news_summary":   newsSummary,
		"device_type":    device.DeviceType,
	})
}
