package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/platform/apperror"
)

func OriginProtection(allowedOrigin string) echo.MiddlewareFunc {
	allowed := canonicalOrigin(allowedOrigin)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}
			origin := c.Request().Header.Get("Origin")
			if origin == "" {
				return apperror.New(http.StatusForbidden, "ORIGIN_REQUIRED", "修改请求必须包含Origin")
			}
			if allowed == "" || canonicalOrigin(origin) != allowed {
				return apperror.New(http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "请求来源不被允许")
			}
			return next(c)
		}
	}
}

func canonicalOrigin(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Host == "" ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return scheme + "://" + strings.ToLower(parsed.Host)
}
