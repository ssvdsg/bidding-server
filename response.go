// HTTP响应处理模块
// 包含所有HTTP响应相关的辅助函数
package main

import (
	"encoding/json"
	"net/http"
)

// ========== JSON响应 ==========

// respondJSON 返回JSON响应
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError 返回错误响应
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// ========== 特殊响应 ==========

// respondEmptyCompanySummary 返回空的公司摘要响应
func respondEmptyCompanySummary(w http.ResponseWriter, page, size int) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "暂无公司中标历史数据",
		"data": map[string]interface{}{
			"items": []map[string]interface{}{},
			"total": 0,
			"page":  page,
			"size":  size,
		},
	})
}

// respondEmptyCompanyRecords 返回空的公司记录响应
func respondEmptyCompanyRecords(w http.ResponseWriter, company string, page, size int) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"error_code": 0,
		"message":    "暂无该公司的中标历史数据",
		"data": map[string]interface{}{
			"company": company,
			"items":   []map[string]interface{}{},
			"total":   0,
			"page":    page,
			"size":    size,
		},
	})
}
