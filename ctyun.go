// CTYun 直连聊天客户端模块
// 实现 IAM 登录、会话管理、文件上传、流式聊天功能
// 无需 relay 中转，直接与 eaichat.ctyun.cn 通信
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ─── Constants ──────────────────────────────────────────────────────────────

const (
	ctyDeskOrigin      = "https://desk.ctyun.cn"
	ctyEAIOrigin       = "https://eaichat.ctyun.cn"
	ctyEAIChatRedirect = "https://eaichat.ctyun.cn:443/chat/#/aichat"
	ctyUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36 Edg/148.0.0.0"
	ctyJarPath         = "storage/ctyun_jar.json"
)

var (
	ctyChatURL        = ctyEAIOrigin + "/ai/portal/v3/openai/chat/completions"
	ctyUploadURL      = ctyEAIOrigin + "/ai/portal/v2/vector/upload"
	ctyResultQueryURL = ctyEAIOrigin + "/ai/portal/v2/vector/upload/resultQuery"
	ctyDeskLoginURL   = ctyDeskOrigin + "/cloudB/dy/iam/api/auth/iam/login"
	ctyCasBootstrap   = ctyDeskOrigin + "/cloudB/dy/iam/api/auth/iam/cas/login?service=" + url.QueryEscape(ctyEAIChatRedirect)
	ctyTicketAuthURL  = ctyEAIOrigin + "/sso/login/v2/iam/ticketAuthorize"
)

// ─── Types ──────────────────────────────────────────────────────────────────

// CTYunUploadedFile 上传文件解析结果
type CTYunUploadedFile struct {
	FileID    string `json:"file_id"`
	FileType  string `json:"file_type,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	WordCount int    `json:"word_count,omitempty"`
	FileSize  int    `json:"file_size,omitempty"`
	Raw       any    `json:"raw,omitempty"`
}

// CTYunChatRequest 聊天请求参数
type CTYunChatRequest struct {
	Prompt         string
	Files          []CTYunUploadedFile
	Model          string
	WebSearch      bool
	EnableThinking bool
}

// CTYunClient CTYun 直连客户端
type CTYunClient struct {
	mu          sync.Mutex
	httpClient  *http.Client
	user        string
	password    string
	jarPath     string
	autoRelogin bool
	stopRenew   chan struct{}
}

// CTYunSSEEvent SSE 事件结构
type CTYunSSEEvent struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content,omitempty"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"delta"`
	} `json:"choices"`
}

// ─── 内部辅助 ───────────────────────────────────────────────────────────────

func ctyRandomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func ctySHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func ctyJWTExp(token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return 0, errors.New("not a JWT")
	}
	payload := parts[1]
	switch len(payload) % 4 {
	case 1:
		payload += "==="
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return 0, fmt.Errorf("decode jwt payload: %w", err)
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return 0, err
	}
	return claims.Exp, nil
}

func ctyMakeEAIHeaders(xuid string) http.Header {
	h := http.Header{}
	traceID := ctyRandomHex(16)
	h.Set("accept", "application/json, text/plain, */*")
	h.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	h.Set("origin", ctyEAIOrigin)
	h.Set("referer", ctyEAIOrigin+"/chat/")
	h.Set("user-agent", ctyUserAgent)
	h.Set("x-user-agent", ctyUserAgent)
	h.Set("x-client-trace-id", traceID)
	h.Set("x-eai-env", "pubWeb")
	h.Set("x-eai-env-code", "")
	h.Set("x-eai-gw-code", "")
	h.Set("x-eai-mode", "eai")
	h.Set("x-eai-source", "web-eai")
	h.Set("x-eai-tenant-id", "15")
	h.Set("x-eai-version", "202060101")
	h.Set("x-eai-xuid", xuid)
	h.Set("yl-main-version", "202060101")
	h.Set("yl-product-id", "5")
	return h
}

func ctyMimeType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	case ".doc", ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls", ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt", ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".py":
		return "text/x-python"
	case ".js":
		return "text/javascript"
	case ".ts":
		return "text/typescript"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// ─── 会话管理 ───────────────────────────────────────────────────────────────

// isSessionAlive 检查 cookie jar 中的 YL-Token 是否有效
func (c *CTYunClient) isSessionAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	u, _ := url.Parse(ctyEAIOrigin)
	cookies := c.httpClient.Jar.Cookies(u)
	var ylToken string
	for _, cookie := range cookies {
		if cookie.Name == "YL-Token" {
			ylToken = cookie.Value
			break
		}
	}
	if ylToken == "" {
		return false
	}
	exp, err := ctyJWTExp(ylToken)
	if err != nil {
		return false
	}
	return exp > time.Now().Unix()+120
}

// SaveJar 持久化 cookie jar 到文件
func (c *CTYunClient) SaveJar() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	u, _ := url.Parse(ctyEAIOrigin)
	cookies := c.httpClient.Jar.Cookies(u)
	cMap := make(map[string]string)
	for _, cookie := range cookies {
		cMap[cookie.Name] = cookie.Value
	}
	store := map[string]any{
		"cookies":  cMap,
		"saved_at": time.Now().Unix(),
	}
	if c.user != "" && c.password != "" {
		store["credentials"] = map[string]string{
			"user":     c.user,
			"password": c.password,
		}
	}
	data, _ := json.MarshalIndent(store, "", "  ")
	os.MkdirAll(filepath.Dir(c.jarPath), 0755)
	return os.WriteFile(c.jarPath, data, 0644)
}

// loadJar 从文件加载 cookie jar
func (c *CTYunClient) loadJar() error {
	data, err := os.ReadFile(c.jarPath)
	if err != nil {
		return err
	}
	var store struct {
		Cookies     map[string]string `json:"cookies"`
		Credentials map[string]string `json:"credentials,omitempty"`
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return err
	}

	u, _ := url.Parse(ctyEAIOrigin)
	jar, _ := cookiejar.New(nil)
	var cookies []*http.Cookie
	for name, value := range store.Cookies {
		cookies = append(cookies, &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: ".ctyun.cn",
			Path:   "/",
		})
	}
	jar.SetCookies(u, cookies)

	c.mu.Lock()
	c.httpClient.Jar = jar
	if c.user == "" && store.Credentials != nil {
		c.user = store.Credentials["user"]
		c.password = store.Credentials["password"]
	}
	c.mu.Unlock()
	return nil
}

// ─── 登录流程 ───────────────────────────────────────────────────────────────

// casBootstrap CAS 引导
func (c *CTYunClient) casBootstrap() error {
	req, err := http.NewRequest("GET", ctyCasBootstrap, nil)
	if err != nil {
		return err
	}
	req.Header.Set("referer", ctyDeskOrigin+"/cloudB/dy/iam/")
	req.Header.Set("user-agent", ctyUserAgent)
	c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() {
		c.httpClient.CheckRedirect = nil
	}()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// deskLogin 桌面端登录，返回 ticket
func (c *CTYunClient) deskLogin(user, password string) (string, error) {
	bodyMap := map[string]string{
		"userAccount": user,
		"password":    ctySHA256Hex(password),
		"deviceCode":  "iam:" + ctyRandomHex(16),
		"deviceName":  "iam:web",
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	req, err := http.NewRequest("POST", ctyDeskLoginURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json;charset=UTF-8")
	req.Header.Set("origin", ctyDeskOrigin)
	req.Header.Set("referer", ctyDeskOrigin+"/cloudB/dy/iam/")
	req.Header.Set("user-agent", ctyUserAgent)
	req.Header.Set("accept", "application/json, text/plain, */*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Code int `json:"code"`
		Data *struct {
			ReturnURL string `json:"returnUrl"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("desk login: non-json: %s", truncateString(string(raw), 300))
	}
	if parsed.Code != 0 && parsed.Code != 200 {
		return "", fmt.Errorf("desk login failed: code=%d msg=%s", parsed.Code, parsed.Msg)
	}
	if parsed.Data == nil || parsed.Data.ReturnURL == "" {
		return "", fmt.Errorf("desk login: no returnUrl")
	}
	ticket := ctyExtractTicket(parsed.Data.ReturnURL)
	if ticket == "" {
		return "", fmt.Errorf("desk login: no ticket in returnUrl: %s", parsed.Data.ReturnURL)
	}
	return ticket, nil
}

func ctyExtractTicket(returnURL string) string {
	if idx := strings.Index(returnURL, "ticket="); idx >= 0 {
		ticket := returnURL[idx+7:]
		if amp := strings.Index(ticket, "&"); amp >= 0 {
			ticket = ticket[:amp]
		}
		return ticket
	}
	return ""
}

// ticketAuthorize 票据授权，设置 YL-Ssid / YL-Token cookie
func (c *CTYunClient) ticketAuthorize(ticket string) error {
	form := url.Values{
		"loginType":   {"iamTicket"},
		"clientId":    {"eaiapp"},
		"iamTicket":   {ticket},
		"redirectUri": {ctyEAIChatRedirect},
	}
	req, err := http.NewRequest("POST", ctyTicketAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("origin", ctyEAIOrigin)
	req.Header.Set("referer", ctyEAIOrigin+"/chat/")
	req.Header.Set("user-agent", ctyUserAgent)
	req.Header.Set("accept", "application/json, text/plain, */*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("ticketAuthorize: non-json: %s", truncateString(string(raw), 300))
	}
	if !parsed.Success {
		return fmt.Errorf("ticketAuthorize failed: %s", truncateString(string(raw), 300))
	}
	return nil
}

// Login 执行完整 IAM 登录流程
func (c *CTYunClient) Login(user, password string) error {
	c.mu.Lock()
	oldJar := c.httpClient.Jar
	// 创建新 HTTP 客户端（cookie jar 在登录过程中会被更新）
	jar, _ := cookiejar.New(nil)
	c.httpClient.Jar = jar
	c.mu.Unlock()

	if err := c.casBootstrap(); err != nil {
		c.mu.Lock()
		c.httpClient.Jar = oldJar
		c.mu.Unlock()
		return fmt.Errorf("CAS bootstrap: %w", err)
	}
	ticket, err := c.deskLogin(user, password)
	if err != nil {
		c.mu.Lock()
		c.httpClient.Jar = oldJar
		c.mu.Unlock()
		return fmt.Errorf("desk login: %w", err)
	}
	if err := c.ticketAuthorize(ticket); err != nil {
		c.mu.Lock()
		c.httpClient.Jar = oldJar
		c.mu.Unlock()
		return fmt.Errorf("ticket authorize: %w", err)
	}

	c.mu.Lock()
	c.user = user
	c.password = password
	c.mu.Unlock()
	c.SaveJar()
	return nil
}

// EnsureSession 确保会话有效，过期则自动重新登录
func (c *CTYunClient) EnsureSession() error {
	if c.isSessionAlive() {
		return nil
	}
	log.Println("[CTYun] 会话已过期，正在重新登录...")
	if c.user == "" || c.password == "" {
		// 尝试从 jar 文件加载凭据
		if err := c.loadJar(); err != nil {
			return errors.New("session expired and no credentials available for auto-relogin")
		}
		if c.user == "" || c.password == "" {
			return errors.New("session expired and no credentials available for auto-relogin")
		}
	}
	if err := c.Login(c.user, c.password); err != nil {
		return fmt.Errorf("auto-relogin failed: %w", err)
	}
	log.Println("[CTYun] 重新登录成功")
	return nil
}

// ─── 文件上传 ───────────────────────────────────────────────────────────────

// UploadFiles 上传多个文件，返回 UploadedFile 列表
func (c *CTYunClient) UploadFiles(filePaths []string, xuid string) ([]CTYunUploadedFile, error) {
	// Step 1: 上传
	fileIDs, err := c.uploadToUpstream(filePaths, xuid)
	if err != nil {
		return nil, err
	}
	log.Printf("[CTYun] 上传成功 file_ids=%v", fileIDs)

	// Step 2: 轮询解析结果
	return c.pollFileResult(fileIDs, xuid)
}

func (c *CTYunClient) uploadToUpstream(filePaths []string, xuid string) ([]string, error) {
	c.mu.Lock()
	client := c.httpClient
	c.mu.Unlock()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	w.WriteField("desk_uuid", xuid)
	w.WriteField("knowledgebase_type", "personal")

	for _, fp := range filePaths {
		f, err := os.Open(fp)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", fp, err)
		}
		filename := filepath.Base(fp)

		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
		h.Set("Content-Type", ctyMimeType(filename))
		part, err := w.CreatePart(h)
		if err != nil {
			f.Close()
			return nil, err
		}
		if _, err := io.Copy(part, f); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
	}
	w.Close()

	req, err := http.NewRequest("POST", ctyUploadURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	hdr := ctyMakeEAIHeaders(xuid)
	for k, vals := range hdr {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("upload HTTP %d: %s", resp.StatusCode, truncateString(string(raw), 500))
	}

	var upResp struct {
		ResultCode int    `json:"resultCode"`
		ResultMsg  string `json:"resultMsg"`
		Data       []struct {
			Success bool   `json:"success"`
			FileID  string `json:"file_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &upResp); err != nil {
		return nil, fmt.Errorf("upload response parse: %w", err)
	}
	if upResp.ResultCode != 0 {
		return nil, fmt.Errorf("upload failed: resultCode=%d msg=%s", upResp.ResultCode, upResp.ResultMsg)
	}

	var fileIDs []string
	for _, item := range upResp.Data {
		if item.Success && item.FileID != "" {
			fileIDs = append(fileIDs, item.FileID)
		}
	}
	if len(fileIDs) == 0 {
		return nil, fmt.Errorf("upload returned no file_ids")
	}
	return fileIDs, nil
}

func (c *CTYunClient) pollFileResult(fileIDs []string, xuid string) ([]CTYunUploadedFile, error) {
	c.mu.Lock()
	client := c.httpClient
	c.mu.Unlock()

	maxAttempts := 25
	interval := 2 * time.Second

	bodyMap := map[string][]string{"file_ids": fileIDs}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		bodyBytes, _ := json.Marshal(bodyMap)
		req, err := http.NewRequest("POST", ctyResultQueryURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("content-type", "application/json;charset=UTF-8")
		hdr := ctyMakeEAIHeaders(xuid)
		for k, vals := range hdr {
			for _, v := range vals {
				req.Header.Set(k, v)
			}
		}
		req.Header.Set("accept", "application/json, text/plain, */*")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("result query HTTP %d: %s", resp.StatusCode, truncateString(string(raw), 500))
		}

		var qryResp struct {
			ResultCode int    `json:"resultCode"`
			ResultMsg  string `json:"resultMsg"`
			Data       []struct {
				Success      bool   `json:"success"`
				FileID       string `json:"file_id"`
				FileName     string `json:"file_name"`
				FileType     string `json:"file_type"`
				FileSize     int    `json:"file_size"`
				WordCount    int    `json:"word_count"`
				VectorStatus int    `json:"vectorStatus"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &qryResp); err != nil {
			return nil, fmt.Errorf("result query parse: %w", err)
		}
		if qryResp.ResultCode != 0 {
			if attempt < maxAttempts {
				time.Sleep(interval)
				continue
			}
			return nil, fmt.Errorf("result query failed: resultCode=%d msg=%s", qryResp.ResultCode, qryResp.ResultMsg)
		}

		allDone := len(qryResp.Data) > 0
		var results []CTYunUploadedFile
		for _, item := range qryResp.Data {
			if !item.Success || item.FileID == "" || item.VectorStatus != 3 {
				allDone = false
				break
			}
			results = append(results, CTYunUploadedFile{
				FileID:    item.FileID,
				FileType:  item.FileType,
				FileName:  item.FileName,
				WordCount: item.WordCount,
				FileSize:  item.FileSize,
				Raw:       struct{}{},
			})
		}
		if allDone {
			return results, nil
		}

		if attempt < maxAttempts {
			time.Sleep(interval)
		}
	}

	return nil, fmt.Errorf("file parse timeout after %d attempts", maxAttempts)
}

// ─── 流式聊天 ───────────────────────────────────────────────────────────────

// StreamChat 执行流式聊天，通过 callback 实时输出思考内容和回答
// reasoningCb: 思考内容回调（每次有新的 reasoning_content 时调用）
// contentCb: 回答内容回调（每次有新的 content 时调用）
// 返回完整文本
func (c *CTYunClient) StreamChat(
	req CTYunChatRequest,
	xuid string,
	reasoningCb func(string),
	contentCb func(string),
) (string, error) {
	c.mu.Lock()
	client := c.httpClient
	c.mu.Unlock()

	xuidVal := xuid
	if xuidVal == "" {
		xuidVal = "pubweb_" + ctyRandomHex(16)
	}

	msgFiles := req.Files
	if msgFiles == nil {
		msgFiles = []CTYunUploadedFile{}
	}

	bodyMap := map[string]any{
		"key_model": req.Model,
		"messages": []map[string]any{
			{
				"role":      "user",
				"content":   req.Prompt,
				"verify_id": ctyRandomHex(16),
				"ref": map[string]any{
					"type": "file",
					"file": msgFiles,
				},
			},
		},
		"stream":          true,
		"client_retry":    true,
		"web_search":      req.WebSearch,
		"tenantId":        15,
		"enable_thinking": req.EnableThinking,
	}

	bodyBytes, _ := json.Marshal(bodyMap)
	httpReq, err := http.NewRequest("POST", ctyChatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "*/*")
	hdr := ctyMakeEAIHeaders(xuidVal)
	for k, vals := range hdr {
		for _, v := range vals {
			httpReq.Header.Set(k, v)
		}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat HTTP %d: %s", resp.StatusCode, truncateString(string(raw), 500))
	}

	var fullBuilder strings.Builder
	reader := bufio.NewReader(resp.Body)
	thinkingStarted := false
	thinkingDone := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fullBuilder.String(), err
		}

		rawLine := strings.TrimSpace(line)
		if rawLine == "" || !strings.HasPrefix(rawLine, "data:") {
			continue
		}

		payload := strings.TrimSpace(rawLine[5:])
		if payload == "[DONE]" {
			break
		}

		var evt CTYunSSEEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}

		for _, choice := range evt.Choices {
			if choice.Delta.ReasoningContent != "" {
				if !thinkingStarted {
					thinkingStarted = true
				}
				fullBuilder.WriteString(choice.Delta.ReasoningContent)
				if reasoningCb != nil {
					reasoningCb(choice.Delta.ReasoningContent)
				}
			}
			if choice.Delta.Content != "" {
				if thinkingStarted && !thinkingDone {
					thinkingDone = true
				}
				fullBuilder.WriteString(choice.Delta.Content)
				if contentCb != nil {
					contentCb(choice.Delta.Content)
				}
			}
		}
	}

	return strings.TrimSpace(fullBuilder.String()), nil
}

// ─── 自动续期 ───────────────────────────────────────────────────────────────

// StartAutoRenew 启动后台自动续期协程
// 每 5 分钟检查一次会话状态，过期自动重新登录
func (c *CTYunClient) StartAutoRenew() {
	c.stopRenew = make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		log.Println("[CTYun] 自动续期协程已启动（每5分钟检查一次）")
		for {
			select {
			case <-ticker.C:
				if !c.isSessionAlive() {
					log.Println("[CTYun] 检测到会话即将过期，执行续期...")
					if err := c.EnsureSession(); err != nil {
						log.Printf("[CTYun] 续期失败: %v", err)
					} else {
						log.Println("[CTYun] 续期成功")
					}
				}
			case <-c.stopRenew:
				log.Println("[CTYun] 自动续期协程已停止")
				return
			}
		}
	}()
}

// StopAutoRenew 停止自动续期
func (c *CTYunClient) StopAutoRenew() {
	if c.stopRenew != nil {
		close(c.stopRenew)
	}
}

// ─── 构造/初始化 ────────────────────────────────────────────────────────────

// NewCTYunClient 创建 CTYun 客户端
// 从 jar 文件恢复会话，或从环境变量获取凭据
func NewCTYunClient() *CTYunClient {
	jar, _ := cookiejar.New(nil)
	c := &CTYunClient{
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 0,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
		jarPath:  filepath.Join(storageDir, "ctyun_jar.json"),
		user:     getEnv("CTYUN_USER", ""),
		password: getEnv("CTYUN_PASSWORD", ""),
	}

	// 尝试从 jar 文件恢复会话
	if err := c.loadJar(); err == nil {
		if c.isSessionAlive() {
			log.Println("[CTYun] 从 jar 文件恢复会话成功")
		}
	}

	return c
}

// ─── 工具函数 ───────────────────────────────────────────────────────────────

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ─── HTTP Handlers (注册在 main.go 的 /api/chat/* 路由上) ─────────────────

// ctyunChatModels 列出可用的模型列表
func ctyunChatModels(w http.ResponseWriter, r *http.Request) {
	models := []map[string]string{
		{"id": "TEXT_DEEPSEEK_V4", "name": "deepseek-v4", "desc": "默认模型"},
		{"id": "TEXT_A14", "name": "glm-5", "desc": "智谱 GLM-5"},
		{"id": "TEXT_A22", "name": "qwen3.5-plus", "desc": "千问 3.5 plus"},
		{"id": "TEXT_A13", "name": "qwen3:30b", "desc": "千问 3 30B"},
		{"id": "TEXT_A8", "name": "deepseek-v32", "desc": "DeepSeek V3.2"},
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       models,
	})
}

// ctyunChatUpload 上传文件到 CTYun，返回 UploadedFile 列表
func ctyunChatUpload(w http.ResponseWriter, r *http.Request) {
	if ctyunClient == nil {
		respondError(w, http.StatusInternalServerError, "CTYun 客户端未初始化")
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "解析上传表单失败: "+err.Error())
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		respondError(w, http.StatusBadRequest, "未找到上传文件")
		return
	}

	var filePaths []string
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "打开上传文件失败: "+err.Error())
			return
		}
		tmpPath := filepath.Join(os.TempDir(), "ctyun_"+fh.Filename)
		dst, err := os.Create(tmpPath)
		if err != nil {
			f.Close()
			respondError(w, http.StatusInternalServerError, "创建临时文件失败: "+err.Error())
			return
		}
		if _, err := io.Copy(dst, f); err != nil {
			f.Close()
			dst.Close()
			respondError(w, http.StatusInternalServerError, "写入临时文件失败: "+err.Error())
			return
		}
		f.Close()
		dst.Close()
		filePaths = append(filePaths, tmpPath)
	}

	if err := ctyunClient.EnsureSession(); err != nil {
		respondError(w, http.StatusInternalServerError, "CTYun 会话失效: "+err.Error())
		return
	}

	xuid := "pubweb_" + ctyRandomHex(16)
	uploadedFiles, err := ctyunClient.UploadFiles(filePaths, xuid)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "上传文件到 CTYun 失败: "+err.Error())
		return
	}

	for _, p := range filePaths {
		os.Remove(p)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       uploadedFiles,
	})
}

// ctyunChatStream 流式聊天（SSE 转发）
func ctyunChatStream(w http.ResponseWriter, r *http.Request) {
	if ctyunClient == nil {
		respondError(w, http.StatusInternalServerError, "CTYun 客户端未初始化")
		return
	}

	var req struct {
		Prompt         string              `json:"prompt"`
		Files          []CTYunUploadedFile `json:"files,omitempty"`
		Model          string              `json:"model"`
		WebSearch      bool                `json:"web_search"`
		EnableThinking bool                `json:"enable_thinking"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效请求: "+err.Error())
		return
	}
	if req.Prompt == "" {
		respondError(w, http.StatusBadRequest, "prompt 不能为空")
		return
	}
	if req.Model == "" {
		req.Model = "TEXT_DEEPSEEK_V4"
	}

	if err := ctyunClient.EnsureSession(); err != nil {
		respondError(w, http.StatusInternalServerError, "CTYun 会话失效: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := w.(http.Flusher)

	xuid := "pubweb_" + ctyRandomHex(16)
	msgFiles := req.Files
	if msgFiles == nil {
		msgFiles = []CTYunUploadedFile{}
	}

	// Build upstream request inline
	upstreamBody := map[string]interface{}{
		"key_model": req.Model,
		"messages": []map[string]interface{}{
			{
				"role":      "user",
				"content":   req.Prompt,
				"verify_id": ctyRandomHex(16),
				"ref": map[string]interface{}{
					"type": "file",
					"file": msgFiles,
				},
			},
		},
		"stream":          true,
		"client_retry":    true,
		"web_search":      req.WebSearch,
		"tenantId":        15,
		"enable_thinking": req.EnableThinking,
	}

	bodyBytes, _ := json.Marshal(upstreamBody)
	upstreamReq, _ := http.NewRequest("POST", ctyChatURL, bytes.NewReader(bodyBytes))
	upstreamReq.Header.Set("content-type", "application/json")
	upstreamReq.Header.Set("accept", "*/*")
	hdr := ctyMakeEAIHeaders(xuid)
	for k, vals := range hdr {
		for _, v := range vals {
			upstreamReq.Header.Set(k, v)
		}
	}

	// 使用客户端 context，断开时自动取消上游请求
	upstreamReq = upstreamReq.WithContext(r.Context())
	upstreamResp, err := ctyunClient.httpClient.Do(upstreamReq)
	if err != nil {
		respondError(w, http.StatusBadGateway, "上游请求失败: "+err.Error())
		return
	}
	defer upstreamResp.Body.Close()

	if upstreamResp.StatusCode != 200 {
		raw, _ := io.ReadAll(upstreamResp.Body)
		respondError(w, http.StatusBadGateway, fmt.Sprintf("上游 %d: %s", upstreamResp.StatusCode, truncateString(string(raw), 500)))
		return
	}

	// ── SSE 读循环 + panic 恢复 ──
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[CTYun流式] 转发异常: %v", rec)
			fmt.Fprintf(w, "data: [DONE_WITH_ERROR]\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
	}()

	reader := bufio.NewReader(upstreamResp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		rawLine := strings.TrimSpace(line)
		if rawLine == "" || !strings.HasPrefix(rawLine, "data:") {
			continue
		}
		payload := strings.TrimSpace(rawLine[5:])
		if payload == "[DONE]" {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			break
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		if flusher != nil {
			flusher.Flush()
		}
	}
}
