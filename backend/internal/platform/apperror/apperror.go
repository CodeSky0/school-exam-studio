package apperror

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Error struct {
	HTTPStatus int
	Code       string
	Message    string
	Details    []FieldError
	Cause      error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func New(status int, code, message string) *Error {
	return &Error{HTTPStatus: status, Code: code, Message: message, Details: []FieldError{}}
}

func Validation(details []FieldError) *Error {
	if details == nil {
		details = []FieldError{}
	}
	return &Error{
		HTTPStatus: http.StatusBadRequest,
		Code:       "VALIDATION_ERROR",
		Message:    "请求内容未通过校验",
		Details:    details,
	}
}

func Internal(cause error) *Error {
	error := New(http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
	error.Cause = cause
	return error
}

type errorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details"`
}

type errorResponse struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id"`
}

func HTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	applicationError := normalize(err)
	if applicationError.HTTPStatus >= http.StatusInternalServerError {
		c.Logger().Errorf("request_id=%s status=%d error=%v", requestID(c), applicationError.HTTPStatus, err)
	}
	details := applicationError.Details
	if details == nil {
		details = []FieldError{}
	}
	response := errorResponse{
		Error: errorBody{
			Code:    applicationError.Code,
			Message: applicationError.Message,
			Details: details,
		},
		RequestID: requestID(c),
	}
	if jsonError := c.JSON(applicationError.HTTPStatus, response); jsonError != nil {
		c.Logger().Error(jsonError)
	}
}

func normalize(err error) *Error {
	var applicationError *Error
	if errors.As(err, &applicationError) {
		return applicationError
	}
	var httpError *echo.HTTPError
	if errors.As(err, &httpError) {
		return fromHTTPStatus(httpError.Code)
	}
	return Internal(err)
}

func fromHTTPStatus(status int) *Error {
	switch status {
	case http.StatusBadRequest:
		return New(status, "BAD_REQUEST", "请求格式错误")
	case http.StatusUnauthorized:
		return New(status, "UNAUTHORIZED", "未认证或会话已失效")
	case http.StatusForbidden:
		return New(status, "FORBIDDEN", "没有操作权限")
	case http.StatusNotFound:
		return New(status, "NOT_FOUND", "资源不存在")
	case http.StatusMethodNotAllowed:
		return New(status, "METHOD_NOT_ALLOWED", "请求方法不允许")
	case http.StatusConflict:
		return New(status, "CONFLICT", "请求冲突")
	case http.StatusRequestEntityTooLarge:
		return New(status, "REQUEST_TOO_LARGE", "请求内容过大")
	case http.StatusUnsupportedMediaType:
		return New(status, "UNSUPPORTED_MEDIA_TYPE", "请求媒体类型不受支持")
	case http.StatusUnprocessableEntity:
		return New(status, "UNPROCESSABLE_ENTITY", "请求内容无法处理")
	case http.StatusTooManyRequests:
		return New(status, "RATE_LIMITED", "请求过于频繁，请稍后重试")
	case http.StatusServiceUnavailable:
		return New(status, "SERVICE_UNAVAILABLE", "服务暂不可用")
	case http.StatusGatewayTimeout:
		return New(status, "GATEWAY_TIMEOUT", "上游服务超时")
	default:
		return Internal(nil)
	}
}

func requestID(c echo.Context) string {
	id := c.Response().Header().Get(echo.HeaderXRequestID)
	if id == "" {
		id = c.Request().Header.Get(echo.HeaderXRequestID)
	}
	if id == "" {
		id = uuid.NewString()
		c.Response().Header().Set(echo.HeaderXRequestID, id)
	}
	return id
}
