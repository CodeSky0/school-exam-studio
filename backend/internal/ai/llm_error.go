package ai

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrorKind 描述模型调用失败的可恢复性，供弹性运行时决定重试、切换模型还是直接失败。
type ErrorKind string

const (
	// ErrorKindConfig 表示配置或鉴权问题，重试无意义。
	ErrorKindConfig ErrorKind = "config"
	// ErrorKindRetryable 表示限流、超时、网络或服务端临时错误，可重试。
	ErrorKindRetryable ErrorKind = "retryable"
	// ErrorKindFatal 表示请求本身或内容策略问题，重试无意义。
	ErrorKindFatal ErrorKind = "fatal"
	// ErrorKindParse 表示模型返回内容无法解析为 JSON，可通过修复提示重试。
	ErrorKindParse ErrorKind = "parse"
)

func classifyStatus(status int) ErrorKind {
	switch {
	case status == http.StatusUnauthorized,
		status == http.StatusForbidden,
		status == http.StatusNotFound:
		return ErrorKindConfig
	case status == http.StatusRequestTimeout,
		status == http.StatusConflict,
		status == http.StatusTooManyRequests,
		status == http.StatusBadGateway,
		status == http.StatusServiceUnavailable,
		status == http.StatusGatewayTimeout,
		status >= http.StatusInternalServerError:
		return ErrorKindRetryable
	default:
		return ErrorKindFatal
	}
}

// retryAfterFromHeader 解析 Retry-After 的秒数形式，无法识别时返回 0。
func retryAfterFromHeader(header http.Header) time.Duration {
	if header == nil {
		return 0
	}
	raw := strings.TrimSpace(header.Get("Retry-After"))
	if raw == "" {
		return 0
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
