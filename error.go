package codesignapi

import "fmt"

// APIError 表示 HTTP 非 2xx 或业务状态码非零，调用方可通过 errors.As 提取。
// 无法解析 HTTP 错误响应时，Code 和 Message 为零值，应同时判断 StatusCode。
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Code       int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("codesign: %s %s failed: status=%d, code=%d, message=%q", e.Method, e.Path, e.StatusCode, e.Code, e.Message)
}
