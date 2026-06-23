// 工具函数模块
// 包含各种通用工具函数：字符串处理、编码解码、数据转换等
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ========== 环境变量和配置 ==========

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getConfigValue(key, defaultValue string) string {
	if db != nil {
		var value sql.NullString
		err := db.QueryRow("SELECT value FROM configs WHERE `key` = ? LIMIT 1", key).Scan(&value)
		if err == nil && value.Valid && strings.TrimSpace(value.String) != "" {
			return strings.TrimSpace(value.String)
		}
	}
	return getEnv(key, defaultValue)
}

// parseInt 解析字符串为整数，失败时返回默认值
func parseInt(val string, def int) int {
	if val == "" {
		return def
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	return def
}

// ========== ID编码解码 ==========

// encodeBidID 编码招标ID，将特殊字符进行URL编码
func encodeBidID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return strings.ReplaceAll(id, "=", "%3D")
}

// decodeBidID 解码招标ID，将URL编码还原
func decodeBidID(id string) string {
	if id == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(id)
	if err != nil {
		return strings.ReplaceAll(id, "%3D", "=")
	}
	return decoded
}

// ========== JSON处理 ==========

// decodeJSONBody 解码HTTP请求体为JSON
func decodeJSONBody(r *http.Request) (map[string]interface{}, error) {
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var payload map[string]interface{}
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// toJSONString 将对象转换为JSON字符串
func toJSONString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// ========== 数据提取函数 ==========

// pickString 从map中提取字符串值，支持多个候选键
func pickString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case string:
				trimmed := strings.TrimSpace(v)
				if trimmed != "" {
					return trimmed
				}
			case json.Number:
				return v.String()
			case float64:
				return strconv.FormatFloat(v, 'f', -1, 64)
			case bool:
				return strconv.FormatBool(v)
			}
		}
	}
	return ""
}

// pickFloat 从map中提取浮点数，支持多个候选键
func pickFloat(data map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case float64:
				return v
			case json.Number:
				if f, err := v.Float64(); err == nil {
					return f
				}
			case string:
				parsed := strings.ReplaceAll(strings.TrimSpace(v), ",", "")
				if parsed == "" {
					continue
				}
				if f, err := strconv.ParseFloat(parsed, 64); err == nil {
					return f
				}
			}
		}
	}
	return 0
}

// pickInt 从map中提取整数，支持多个候选键
func pickInt(data map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case float64:
				return int64(v)
			case json.Number:
				if i, err := v.Int64(); err == nil {
					return i
				}
			case string:
				parsed := strings.TrimSpace(v)
				if parsed == "" {
					continue
				}
				if i, err := strconv.ParseInt(parsed, 10, 64); err == nil {
					return i
				}
			}
		}
	}
	return 0
}

// ========== 时间处理 ==========

// parseTimeToUnix 解析时间字符串为Unix时间戳
func parseTimeToUnix(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if ts, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return ts.Unix()
		}
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return t.Unix()
	}
	return 0
}

// formattedFetchTime 格式化当前时间为标准格式
func formattedFetchTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// normalizeFetchTime 规范化获取时间字符串
func normalizeFetchTime(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return formattedFetchTime()
	}
	if timeOnlyPattern.MatchString(trimmed) {
		return time.Now().Format("2006-01-02") + " " + trimmed
	}
	return trimmed
}

// ========== 字符串处理 ==========

// sanitizeFileToken 清理文件令牌字符串，只保留字母数字和连字符
func sanitizeFileToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	sanitized := strings.Trim(builder.String(), "-_")
	return sanitized
}

// truncateRunes 截断字符串到指定字符数（按rune计算）
func truncateRunes(input string, limit int) string {
	if limit <= 0 || input == "" {
		return ""
	}
	if utf8.RuneCountInString(input) <= limit {
		return input
	}
	runes := []rune(input)
	if limit > len(runes) {
		limit = len(runes)
	}
	return string(runes[:limit])
}

// stripHTMLCompact 移除HTML标签，压缩空白字符
func stripHTMLCompact(input string) string {
	if input == "" {
		return ""
	}
	text := scriptTagPattern.ReplaceAllString(input, "")
	text = styleTagPattern.ReplaceAllString(text, "")
	text = htmlCommentPattern.ReplaceAllString(text, "")
	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = whitespacePattern.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// stripHTMLPreserveBreaks 移除HTML标签，保留换行
func stripHTMLPreserveBreaks(input string) string {
	if input == "" {
		return ""
	}
	text := scriptTagPattern.ReplaceAllString(input, "")
	text = styleTagPattern.ReplaceAllString(text, "")
	text = htmlCommentPattern.ReplaceAllString(text, "")
	text = newlineTagPattern.ReplaceAllString(text, "\n")
	text = htmlTagPattern.ReplaceAllString(text, "\n")
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\u00a0", " ")
	lines := strings.Split(text, "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	return strings.Join(clean, "\n\n")
}

// defaultString 如果字符串为空则返回默认值
func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// firstNonEmpty 返回第一个非空字符串
func firstNonEmpty(values ...string) string {
	for _, val := range values {
		if strings.TrimSpace(val) != "" {
			return val
		}
	}
	return ""
}

// ========== SQL Null值处理 ==========

// nullStringValue 将sql.NullString转换为字符串
func nullStringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullFloatValue 将sql.NullFloat64转换为float64
func nullFloatValue(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}

// nullString 将字符串转换为sql.NullString
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// ========== 金额转换 ==========

// convertAmount 将金额转换为可读格式
func convertAmount(amount float64) string {
	if amount >= 1e8 { // 亿
		return fmt.Sprintf("%.2f亿元", amount/1e8)
	} else if amount >= 1e7 { // 千万
		return fmt.Sprintf("%.2f千万元", amount/1e7)
	} else if amount >= 1e6 { // 百万
		return fmt.Sprintf("%.2f百万元", amount/1e6)
	} else if amount >= 1e4 { // 万
		return fmt.Sprintf("%.2f万元", amount/1e4)
	} else if amount >= 1e3 { // 千
		return fmt.Sprintf("%.2f千元", amount/1e3)
	} else { // 元
		return fmt.Sprintf("%.2f元", amount)
	}
}

// ========== 工具函数 ==========

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isTableMissing 检查错误是否为表不存在
func isTableMissing(err error, tableName string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "doesn't exist") && strings.Contains(msg, strings.ToLower(tableName))
}

// cleanPathMiddleware 清理URL路径中的重复斜杠
func cleanPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" {
			next.ServeHTTP(w, r)
			return
		}
		cleaned := path.Clean(r.URL.Path)
		if cleaned == "." {
			cleaned = "/"
		}
		if cleaned != r.URL.Path {
			r.URL.Path = cleaned
		}
		next.ServeHTTP(w, r)
	})
}

// ========== 数据库查询辅助 ==========

// queryRows 执行查询并返回map列表
func queryRows(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		scans := make([]interface{}, len(columns))
		for i := range values {
			scans[i] = &values[i]
		}

		if err := rows.Scan(scans...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			switch v := values[i].(type) {
			case nil:
				rowMap[col] = nil
			case []byte:
				rowMap[col] = string(v)
			default:
				rowMap[col] = v
			}
		}

		results = append(results, rowMap)
	}

	return results, nil
}

// queryCount 执行COUNT查询并返回整数
func queryCount(query string, args ...interface{}) (int, error) {
	var total sql.NullInt64
	if err := db.QueryRow(query, args...).Scan(&total); err != nil {
		return 0, err
	}
	if total.Valid {
		return int(total.Int64), nil
	}
	return 0, nil
}

// queryRowsContext context-aware 的 queryRows
func queryRowsContext(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}

		results = append(results, rowMap)
	}

	return results, nil
}

// queryCountContext context-aware 的 queryCount
func queryCountContext(ctx context.Context, query string, args ...interface{}) (int, error) {
	var total sql.NullInt64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	if total.Valid {
		return int(total.Int64), nil
	}
	return 0, nil
}
