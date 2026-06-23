package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type relayChatRequest struct {
	Prompt         string                   `json:"prompt,omitempty"`
	KeyModel       string                   `json:"key_model,omitempty"`
	Files          []map[string]interface{} `json:"files,omitempty"`
	Stream         bool                     `json:"stream"`
	WebSearch      bool                     `json:"web_search"`
	EnableThinking bool                     `json:"enable_thinking"`
}

type relaySharedFile struct {
	FileID     string `json:"file_id"`
	FileName   string `json:"file_name"`
	FileSource string `json:"file_source"`
	URL        string `json:"url"`
}

func callRelayStreamingAI(systemPrompt, userPrompt string) (string, string, error) {
	return callRelayStreamingAIWithFiles(systemPrompt, userPrompt, nil)
}

func getRelayAIFileModel() string {
	model := strings.TrimSpace(getConfigValue("RELAY_AI_FILE_MODEL", ""))
	lowerModel := strings.ToLower(model)
	switch lowerModel {
	case "":
		// 用户没显式配置文件模型 → 跟随普通模型，避免与界面选择不一致
		general := strings.TrimSpace(getConfigValue("RELAY_AI_MODEL", defaultRelayAIModel))
		if general == "" {
			return defaultRelayAIModel
		}
		return general
	case "deep", "default":
		// 旧别名 → DeepSeek-V4
		return defaultRelayAIModel
	}
	return model
}

func relayAIHeaders() map[string]string {
	return map[string]string{
		"X-API-Key": getConfigValue("RELAY_AI_API_KEY", defaultRelayAIAPIKey),
	}
}

func uploadRelayAIFile(localPath string) ([]string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("打开PDF失败: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return nil, fmt.Errorf("创建上传表单失败: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("写入上传内容失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭上传表单失败: %w", err)
	}

	baseURL := strings.TrimRight(getConfigValue("RELAY_AI_BASE_URL", defaultRelayAIBaseURL), "/")
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/files/upload", &body)
	if err != nil {
		return nil, fmt.Errorf("创建文件上传请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for k, v := range relayAIHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上传PDF失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取上传响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("上传PDF失败 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		ExtractedFileIDs []string `json:"extracted_file_ids"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("解析上传响应失败: %w", err)
	}
	if len(result.ExtractedFileIDs) == 0 {
		return nil, errors.New("上传PDF成功但未返回 file_id")
	}
	return result.ExtractedFileIDs, nil
}

func uploadLingxiSharedFile(localPath string) ([]map[string]interface{}, string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return nil, "", fmt.Errorf("创建上传表单失败: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("写入上传内容失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("关闭上传表单失败: %w", err)
	}

	baseURL := strings.TrimRight(getConfigValue("RELAY_AI_BASE_URL", defaultRelayAIBaseURL), "/")
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/lingxi/files/upload-share", &body)
	if err != nil {
		return nil, "", fmt.Errorf("创建灵析上传请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for k, v := range relayAIHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("灵析上传失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取灵析上传响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("灵析上传失败 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		File relaySharedFile `json:"file"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, "", fmt.Errorf("解析灵析上传响应失败: %w", err)
	}
	if strings.TrimSpace(result.File.FileID) == "" {
		return nil, "", errors.New("灵析上传成功但未返回 file_id")
	}

	return []map[string]interface{}{
		{
			"file_id":     result.File.FileID,
			"file_name":   result.File.FileName,
			"file_source": firstNonEmpty(result.File.FileSource, "drive"),
		},
	}, strings.TrimSpace(result.File.URL), nil
}

func waitRelayAIFileReady(fileIDs []string) ([]map[string]interface{}, error) {
	baseURL := strings.TrimRight(getConfigValue("RELAY_AI_BASE_URL", defaultRelayAIBaseURL), "/")
	payload := map[string]interface{}{"file_ids": fileIDs}
	body, _ := json.Marshal(payload)

	for attempt := 0; attempt < 25; attempt++ {
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/files/result", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("创建文件结果请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range relayAIHeaders() {
			req.Header.Set(k, v)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("查询文件解析结果失败: %w", err)
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("读取文件结果失败: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("文件解析结果失败 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		}

		var result struct {
			Ready bool                     `json:"ready"`
			Files []map[string]interface{} `json:"files"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("解析文件结果失败: %w", err)
		}
		if result.Ready && len(result.Files) > 0 {
			return result.Files, nil
		}

		time.Sleep(2 * time.Second)
	}

	return nil, errors.New("等待PDF解析超时")
}

func callRelayStreamingAIWithFiles(systemPrompt, userPrompt string, files []map[string]interface{}) (string, string, error) {
	relayAIInvokeState.mu.Lock()
	defer relayAIInvokeState.mu.Unlock()

	if !relayAIInvokeState.lastInvoke.IsZero() {
		elapsed := time.Since(relayAIInvokeState.lastInvoke)
		if elapsed < 15*time.Second {
			wait := 15*time.Second - elapsed
			log.Printf("[AI限流] 距离上次AI调用仅 %v，等待 %v 后继续", elapsed.Round(time.Millisecond), wait.Round(time.Millisecond))
			time.Sleep(wait)
		}
	}

	baseURL := strings.TrimRight(getConfigValue("RELAY_AI_BASE_URL", defaultRelayAIBaseURL), "/")
	model := strings.TrimSpace(getConfigValue("RELAY_AI_MODEL", defaultRelayAIModel))
	if model == "" {
		model = defaultRelayAIModel
	}
	if len(files) > 0 {
		model = getRelayAIFileModel()
	}

	reqBody := relayChatRequest{
		Prompt:   strings.TrimSpace(systemPrompt + "\n\n" + userPrompt),
		KeyModel: model,
		Files:    files,
		Stream:   true,
		// CTYun 模型（所有 TEXT_*）已强制开启联网搜索 + 深度思考，请求体里的字段会被中转忽略。
		// 这里显式置 true 做声明，对 lingxi 链路也明确表达期望开启，便于排查。
		WebSearch:      true,
		EnableThinking: true,
	}

	rawReq, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("序列化流式AI请求失败: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/chat", bytes.NewReader(rawReq))
	if err != nil {
		return "", "", fmt.Errorf("创建流式AI请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range relayAIHeaders() {
		req.Header.Set(k, v)
	}

	if token := strings.TrimSpace(getConfigValue("RELAY_AI_AUTH_TOKEN", "")); token != "" {
		req.Header.Set("Authorization", token)
	}
	if cookie := strings.TrimSpace(getConfigValue("RELAY_AI_COOKIE", "")); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := streamingAIHTTPClient.Do(req)
	relayAIInvokeState.lastInvoke = time.Now()
	if err != nil {
		return "", "", fmt.Errorf("流式AI请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("relay AI error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	// 流式 SSE：在专门的协程里读取，主协程通过 select 实施"空闲超时"
	// 思路：只要 idleTimeout 内没收到任何新行就主动断开，避免上游连接挂死时一直等到 client.Timeout
	const idleTimeout = 90 * time.Second
	type lineMsg struct {
		text string
		eof  bool
		err  error
	}
	lineCh := make(chan lineMsg, 16)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[RelayAI] SSE 读取异常: %v", rec)
				lineCh <- lineMsg{err: fmt.Errorf("SSE 读取 panic: %v", rec)}
			}
		}()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lineCh <- lineMsg{text: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			lineCh <- lineMsg{err: err}
			return
		}
		lineCh <- lineMsg{eof: true}
	}()

	var builder strings.Builder
	idleTimer := time.NewTimer(idleTimeout)
	defer idleTimer.Stop()

readLoop:
	for {
		select {
		case <-idleTimer.C:
			// 主动断流，让上面的 goroutine 的 scanner.Scan 收到错误退出
			_ = resp.Body.Close()
			return "", "", fmt.Errorf("读取流式AI响应失败: 超过 %s 未收到任何新数据", idleTimeout)
		case msg := <-lineCh:
			if msg.err != nil {
				return "", "", fmt.Errorf("读取流式AI响应失败: %w", msg.err)
			}
			if msg.eof {
				break readLoop
			}
			line := strings.TrimSpace(msg.text)
			if line == "" || !strings.HasPrefix(line, "data:") {
				// 收到心跳/空行也算有进展，重置 idle 计时器
				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				idleTimer.Reset(idleTimeout)
				continue
			}

			// 重置 idle 计时器
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(idleTimeout)

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break readLoop
			}

			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(data), &payload); err == nil {
				if choices, ok := payload["choices"].([]interface{}); ok {
					for _, item := range choices {
						choice, ok := item.(map[string]interface{})
						if !ok {
							continue
						}
						delta, ok := choice["delta"].(map[string]interface{})
						if !ok {
							continue
						}
						content, _ := delta["content"].(string)
						if content != "" {
							builder.WriteString(content)
						}
					}
					continue
				}
			}

			builder.WriteString(data)
		}
	}

	responseText := strings.TrimSpace(builder.String())
	if responseText == "" {
		return "", "", errors.New("流式AI未返回内容")
	}

	cleanResponse, thinking := splitThinkingContent(responseText)
	return cleanResponse, thinking, nil
}

// ─── CTYun 直连 AI 分析 ────────────────────────────────────────────────────

// callDirectCTYunAI 使用 CTYun 直连客户端进行 AI 分析
// callDirectCTYunAI 使用 CTYun 直连进行 AI 分析（纯文本，无文件上传）
// 从数据库 ai_prompt 读取提示词，拼接用户内容后发送
func callDirectCTYunAI(systemPrompt, userPrompt string) (string, string, string, error) {
	if ctyunClient == nil {
		return "", "", "", errors.New("CTYun 客户端未初始化")
	}

	if err := ctyunClient.EnsureSession(); err != nil {
		return "", "", "", fmt.Errorf("CTYun 会话失效: %w", err)
	}

	// 拼接完整提示词：数据库 ai_prompt + 用户内容
	promptText := fetchSystemPrompt()
	finalPrompt := strings.ReplaceAll(promptText, "{{bid_info}}", userPrompt)
	if !strings.Contains(promptText, "{{bid_info}}") {
		finalPrompt = promptText + "\n\n【招标项目信息】\n" + userPrompt
	}

	model := getConfigValue("CTYUN_MODEL", "TEXT_DEEPSEEK_V4")

	xuid := "pubweb_" + ctyRandomHex(16)

	// 构建请求体
	upstreamBody := map[string]interface{}{
		"key_model": model,
		"messages": []map[string]interface{}{
			{
				"role":      "user",
				"content":   finalPrompt,
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
		"enable_thinking": true,
	}

	bodyBytes, _ := json.Marshal(upstreamBody)
	req, _ := http.NewRequest("POST", ctyChatURL, bytes.NewReader(bodyBytes))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "*/*")
	hdr := ctyMakeEAIHeaders(xuid)
	for k, vals := range hdr {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	client := ctyunClient.httpClient
	if client == nil {
		return "", "", "", errors.New("CTYun HTTP 客户端不可用")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("CTYun 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("CTYun 返回 %d: %s", resp.StatusCode, truncateString(string(raw), 500))
	}

	// 流式读取 SSE 响应 — 带 panic 恢复
	var fullBuilder strings.Builder
	var thinkingBuilder strings.Builder
	reader := bufio.NewReader(resp.Body)
	inThinking := false

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[CTYunAI] 流式解析异常: %v", rec)
			}
		}()
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				break
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
					inThinking = true
					thinkingBuilder.WriteString(choice.Delta.ReasoningContent)
				}
				if choice.Delta.Content != "" {
					if inThinking {
						inThinking = false
					}
					fullBuilder.WriteString(choice.Delta.Content)
				}
			}
		}
	}()

	responseText := strings.TrimSpace(fullBuilder.String())
	thinkingText := strings.TrimSpace(thinkingBuilder.String())

	log.Printf("[CTYunAI] 分析完成，模型=%s，响应长度=%d，思考长度=%d",
		model, len(responseText), len(thinkingText))

	return responseText, fmt.Sprintf("CTYun (%s)", model), thinkingText, nil
}

// callDirectCTYunWithFiles 使用 CTYun 直连进行带文件的 AI 分析
func callDirectCTYunWithFiles(systemPrompt, userPrompt string, ctyunFiles []CTYunUploadedFile, model string) (string, string, error) {
	if ctyunClient == nil {
		return "", "", errors.New("CTYun 客户端未初始化")
	}

	// 提示词已在上层处理好，直接发送 userPrompt（可能包含短指令 + 已上传文件）
	// 如果 userPrompt 为空或没有文件，回退到从数据库读取完整提示词
	finalPrompt := userPrompt
	if finalPrompt == "" && len(ctyunFiles) == 0 {
		finalPrompt = fetchSystemPrompt()
		if strings.Contains(finalPrompt, "{{bid_info}}") {
			finalPrompt = strings.ReplaceAll(finalPrompt, "{{bid_info}}", systemPrompt)
		}
	}

	if model == "" {
		model = getConfigValue("CTYUN_MODEL", "TEXT_DEEPSEEK_V4")
	}

	xuid := "pubweb_" + ctyRandomHex(16)
	upstreamBody := map[string]interface{}{
		"key_model": model,
		"messages": []map[string]interface{}{
			{
				"role":      "user",
				"content":   finalPrompt,
				"verify_id": ctyRandomHex(16),
				"ref": map[string]interface{}{
					"type": "file",
					"file": ctyunFiles,
				},
			},
		},
		"stream":          true,
		"client_retry":    true,
		"web_search":      true,
		"tenantId":        15,
		"enable_thinking": true,
	}

	bodyBytes, _ := json.Marshal(upstreamBody)
	req, _ := http.NewRequest("POST", ctyChatURL, bytes.NewReader(bodyBytes))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "*/*")
	hdr := ctyMakeEAIHeaders(xuid)
	for k, vals := range hdr {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	client := ctyunClient.httpClient
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("CTYun 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("CTYun 返回 %d: %s", resp.StatusCode, truncateString(string(raw), 500))
	}

	var fullBuilder strings.Builder
	var thinkingBuilder strings.Builder
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			break
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
				thinkingBuilder.WriteString(choice.Delta.ReasoningContent)
			}
			if choice.Delta.Content != "" {
				fullBuilder.WriteString(choice.Delta.Content)
			}
		}
	}

	responseText := strings.TrimSpace(fullBuilder.String())
	thinkingText := strings.TrimSpace(thinkingBuilder.String())
	log.Printf("[CTYunAI] 带文件分析完成: 响应长度=%d, 思考长度=%d", len(responseText), len(thinkingText))
	return responseText, thinkingText, nil
}
