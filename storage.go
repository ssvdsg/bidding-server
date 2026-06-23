// 文件存储和上传模块
// 包含文件上传、下载、存储管理等功能
package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// ========== HTTP Handlers ==========

// uploadChinaHandler 上传中国政府采购网相关文件
func uploadChinaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "解析表单失败")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少文件字段 file"})
		return
	}
	defer file.Close()
	serial := r.FormValue("serial")
	if serial == "" {
		serial = r.FormValue("id")
	}
	base := sanitizeFileToken(serial)
	if base == "" {
		base = "china"
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".pdf"
	}
	fileName := fmt.Sprintf("china/%s-%d%s", base, time.Now().UnixMilli(), ext)
	if err := saveToStorage(fileName, file); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	url := fmt.Sprintf("/%s", fileName)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data":       map[string]string{"url": url},
	})
}

// uploadChinaByIDHandler 根据项目ID上传中国政府采购网相关文件
func uploadChinaByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "解析表单失败")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少文件字段 file"})
		return
	}
	defer file.Close()
	id := r.FormValue("id")
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error_code": 1, "error_msg": "缺少项目 ID"})
		return
	}
	var project struct {
		ID              string
		Serial          sql.NullString
		Title           sql.NullString
		ChinaBulletinID sql.NullString
	}
	query := "SELECT id, serial, title, china_bulletin_id FROM bids WHERE id = ? LIMIT 1"
	err = db.QueryRow(query, id).Scan(&project.ID, &project.Serial, &project.Title, &project.ChinaBulletinID)
	if err == sql.ErrNoRows {
		err = db.QueryRow("SELECT id, serial, title, china_bulletin_id FROM bids WHERE china_bulletin_id = ? LIMIT 1", id).
			Scan(&project.ID, &project.Serial, &project.Title, &project.ChinaBulletinID)
	}
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error_code": 1, "error_msg": "未找到该项目"})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	base := project.ChinaBulletinID.String
	if base == "" && project.Serial.Valid {
		base = project.Serial.String
	}
	if base == "" && len(project.ID) >= 8 {
		base = project.ID[:8]
	}
	base = sanitizeFileToken(base)
	if base == "" {
		base = "china"
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".pdf"
	}
	fileName := fmt.Sprintf("china/%s-%d%s", base, time.Now().UnixMilli(), ext)
	if err := saveToStorage(fileName, file); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	url := fmt.Sprintf("/%s", fileName)
	if _, err := db.Exec("UPDATE bids SET pdf_url = ? WHERE id = ?", url, project.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"data": map[string]interface{}{
			"url":               url,
			"id":                project.ID,
			"china_bulletin_id": project.ChinaBulletinID.String,
			"serial":            project.Serial.String,
			"title":             project.Title.String,
		},
	})
}

// pdfProxy PDF代理服务，用于跨域访问PDF文件
func pdfProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	pdfURL := r.URL.Query().Get("url")
	bidID := strings.TrimSpace(r.URL.Query().Get("id"))
	fileType := strings.TrimSpace(r.URL.Query().Get("file"))

	if pdfURL == "" && bidID != "" {
		var currentURL, originalURL sql.NullString
		if err := db.QueryRow("SELECT pdf_url, original_pdf_url FROM bids WHERE id = ? LIMIT 1", bidID).Scan(&currentURL, &originalURL); err != nil {
			if err == sql.ErrNoRows {
				respondError(w, http.StatusNotFound, "Bid not found")
				return
			}
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		switch strings.ToLower(fileType) {
		case "original":
			if originalURL.Valid && strings.TrimSpace(originalURL.String) != "" {
				pdfURL = strings.TrimSpace(originalURL.String)
			}
		default:
			if currentURL.Valid && strings.TrimSpace(currentURL.String) != "" {
				pdfURL = strings.TrimSpace(currentURL.String)
			}
		}
		if pdfURL == "" && currentURL.Valid {
			pdfURL = strings.TrimSpace(currentURL.String)
		}
		if pdfURL == "" && originalURL.Valid {
			pdfURL = strings.TrimSpace(originalURL.String)
		}
	}
	if pdfURL == "" {
		respondError(w, http.StatusBadRequest, "Missing PDF URL")
		return
	}
	req, err := http.NewRequest(http.MethodGet, pdfURL, nil)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := storageProxyClient
	resp, err := client.Do(req)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		respondError(w, resp.StatusCode, fmt.Sprintf("Failed to fetch PDF: %s", string(body)))
		return
	}

	reader := bufio.NewReader(resp.Body)
	headBytes, _ := reader.Peek(512)
	contentType := resp.Header.Get("Content-Type")
	isPDF := isLikelyPDFContent(pdfURL, contentType, headBytes)

	for k, vals := range resp.Header {
		keyLower := strings.ToLower(k)
		if keyLower == "content-type" || keyLower == "content-disposition" || keyLower == "x-frame-options" || keyLower == "content-security-policy" || keyLower == "content-length" {
			continue
		}
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}

	// 如果上游没给 Content-Type，根据 URL 后缀和首段字节嗅探，确保浏览器能正确渲染（iframe 预览要求）
	if !isPDF && strings.TrimSpace(contentType) == "" {
		contentType = sniffContentType(pdfURL, headBytes)
	}

	if isPDF {
		w.Header().Set("Content-Type", "application/pdf")
	} else if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	io.Copy(w, reader)
}

// sniffContentType 在上游没给 Content-Type 时根据 URL 后缀和文件首字节做嗅探
func sniffContentType(rawURL string, head []byte) string {
	lowerURL := strings.ToLower(rawURL)
	switch {
	case strings.HasSuffix(lowerURL, ".html"), strings.HasSuffix(lowerURL, ".htm"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(lowerURL, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(lowerURL, ".xml"):
		return "application/xml; charset=utf-8"
	case strings.HasSuffix(lowerURL, ".txt"):
		return "text/plain; charset=utf-8"
	}
	headStr := strings.TrimSpace(strings.ToLower(string(head)))
	if strings.HasPrefix(headStr, "<!doctype html") || strings.HasPrefix(headStr, "<html") || strings.Contains(headStr, "<head>") || strings.Contains(headStr, "<body") {
		return "text/html; charset=utf-8"
	}
	if strings.HasPrefix(headStr, "{") || strings.HasPrefix(headStr, "[") {
		return "application/json; charset=utf-8"
	}
	return ""
}

func isLikelyPDFContent(rawURL, contentType string, head []byte) bool {
	lowerURL := strings.ToLower(rawURL)
	lowerType := strings.ToLower(contentType)

	if strings.Contains(lowerType, "application/pdf") {
		return true
	}
	if strings.HasPrefix(string(head), "%PDF-") {
		return true
	}
	if strings.Contains(lowerURL, ".pdf") {
		return true
	}

	return false
}

// handleFileUpload 处理文件上传（用于取代R2）
func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if uploadToken != "" && r.Header.Get("X-Upload-Token") != uploadToken {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "无效的上传令牌"})
		return
	}

	contentType := r.Header.Get("Content-Type")
	var (
		filename string
		reader   io.Reader
		errorMsg string
	)

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			respondError(w, http.StatusBadRequest, "缺少文件字段")
			return
		}
		defer file.Close()
		filename = header.Filename
		reader = file
	} else {
		filename = r.URL.Query().Get("filename")
		if filename == "" {
			filename = r.Header.Get("X-File-Name")
		}
		if filename == "" {
			errorMsg = "缺少文件名"
		} else {
			reader = r.Body
		}
	}

	if reader == nil {
		if errorMsg == "" {
			errorMsg = "未提供文件内容"
		}
		respondError(w, http.StatusBadRequest, errorMsg)
		return
	}

	if err := saveToStorage(filename, reader); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "上传成功",
		"url":     fmt.Sprintf("/files/%s", filename),
	})
}

// handleFileGet 处理文件下载请求
func handleFileGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["path"]
	if filename == "" {
		filename = vars["filename"]
	}
	if filename == "" {
		respondError(w, http.StatusBadRequest, "缺少文件名")
		return
	}
	fullPath := filepath.Join(storageDir, filepath.Clean(filename))
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err == nil {
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
		return
	}
	io.Copy(w, file)
}

// handleRawPutUpload 处理原始PUT请求上传
func handleRawPutUpload(w http.ResponseWriter, r *http.Request) {
	if uploadToken != "" && r.Header.Get("X-Upload-Token") != uploadToken {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "无效的上传令牌"})
		return
	}
	fileName := strings.TrimPrefix(r.URL.Path, "/")
	if fileName == "" {
		respondError(w, http.StatusBadRequest, "缺少文件名")
		return
	}
	if err := saveToStorage(fileName, r.Body); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "上传成功",
		"url":     fmt.Sprintf("/%s", fileName),
	})
}

// tryServeStoredFile 尝试提供存储的文件
// 如果文件存在则提供服务，否则返回false
func tryServeStoredFile(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/api") {
		return false
	}
	fullPath := filepath.Join(storageDir, filepath.Clean(strings.TrimPrefix(r.URL.Path, "/")))
	file, err := os.Open(fullPath)
	if err != nil {
		return false
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return false
	}

	// 设置响应头
	w.Header().Set("Content-Type", http.DetectContentType(nil))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	w.Header().Set("Access-Control-Allow-Origin", "*")

	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
	return true
}

// ========== 存储辅助函数 ==========

// saveToStorage 保存文件到存储目录
func saveToStorage(filename string, reader io.Reader) error {
	if filename == "" {
		return fmt.Errorf("文件名不能为空")
	}
	safeName := filepath.Clean(filename)
	fullPath := filepath.Join(storageDir, safeName)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	tmpPath := fullPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, reader); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return err
	}
	file.Close()
	return os.Rename(tmpPath, fullPath)
}
