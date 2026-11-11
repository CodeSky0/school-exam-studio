package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/apperror"
)

type Principal = auth.Principal

type TokenParser interface {
	Parse(string) (auth.Principal, error)
}

type UserAuthenticator interface {
	Authenticate(context.Context, auth.Principal) (auth.Principal, error)
}

func Authenticate(tokens TokenParser, users UserAuthenticator) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Request().Cookie(auth.SessionCookieName)
			if err != nil || cookie.Value == "" {
				return unauthorized()
			}
			claimed, err := tokens.Parse(cookie.Value)
			if err != nil {
				return unauthorized()
			}
			principal, err := users.Authenticate(c.Request().Context(), claimed)
			if errors.Is(err, auth.ErrSessionInvalid) {
				return unauthorized()
			}
			if err != nil {
				return apperror.Internal(err)
			}
			c.Set("auth.principal", principal)
			return next(c)
		}
	}
}

func PrincipalFrom(c echo.Context) (Principal, bool) {
	principal, ok := c.Get("auth.principal").(Principal)
	return principal, ok
}

func unauthorized() error {
	return apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
}
