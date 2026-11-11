package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/platform/apperror"
)

const SessionCookieName = "school_exam_session"

const maxAuthBodyBytes = 64 << 10

type Handler struct {
	service      *Service
	secureCookie bool
}

func NewHandler(service *Service, secureCookie bool) *Handler {
	return &Handler{service: service, secureCookie: secureCookie}
}

func (h *Handler) InitializationStatus(c echo.Context) error {
	initialized, err := h.service.InitializationStatus(c.Request().Context())
	if err != nil {
		return apperror.Internal(err)
	}
	return c.JSON(http.StatusOK, map[string]bool{"initialized": initialized})
}

func (h *Handler) Initialize(c echo.Context) error {
	var request struct {
		SchoolName string `json:"school_name"`
		AdminName  string `json:"admin_name"`
		Email      string `json:"email"`
		Password   string `json:"password"`
	}
	if err := decodeJSON(c, &request); err != nil {
		return err
	}
	result, err := h.service.Initialize(c.Request().Context(), InitializeInput{
		SchoolName: request.SchoolName,
		AdminName:  request.AdminName,
		Email:      request.Email,
		Password:   request.Password,
	})
	if err != nil {
		return serviceError(err)
	}
	return c.JSON(http.StatusCreated, result)
}

func (h *Handler) Register(c echo.Context) error {
	var request struct {
		SchoolCode string `json:"school_code"`
		Name       string `json:"name"`
		Subject    string `json:"subject"`
		Email      string `json:"email"`
		Password   string `json:"password"`
	}
	if err := decodeJSON(c, &request); err != nil {
		return err
	}
	teacher, err := h.service.Register(c.Request().Context(), RegisterInput{
		SchoolCode: request.SchoolCode,
		Name:       request.Name,
		Subject:    request.Subject,
		Email:      request.Email,
		Password:   request.Password,
	})
	if err != nil {
		return serviceError(err)
	}
	return c.JSON(http.StatusCreated, map[string]User{"user": teacher})
}

func (h *Handler) Login(c echo.Context) error {
	var request struct {
		SchoolCode string `json:"school_code"`
		Email      string `json:"email"`
		Password   string `json:"password"`
	}
	if err := decodeJSON(c, &request); err != nil {
		return err
	}
	result, err := h.service.Login(c.Request().Context(), LoginInput{
		SchoolCode: request.SchoolCode,
		Email:      request.Email,
		Password:   request.Password,
	})
	if err != nil {
		return serviceError(err)
	}
	h.setSessionCookie(c, result.Token)
	return c.JSON(http.StatusOK, map[string]User{"user": result.User})
}

func (h *Handler) Logout(c echo.Context) error {
	h.clearSessionCookie(c)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Me(c echo.Context) error {
	principal, ok := PrincipalFromContext(c)
	if !ok {
		return apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
	}
	currentUser, err := h.service.Me(c.Request().Context(), principal)
	if err != nil {
		return serviceError(err)
	}
	return c.JSON(http.StatusOK, map[string]User{"user": currentUser})
}

func (h *Handler) ChangePassword(c echo.Context) error {
	principal, ok := PrincipalFromContext(c)
	if !ok {
		return apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
	}
	var request struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(c, &request); err != nil {
		return err
	}
	err := h.service.ChangePassword(c.Request().Context(), principal, ChangePasswordInput{
		OldPassword: request.OldPassword,
		NewPassword: request.NewPassword,
	})
	if err != nil {
		return serviceError(err)
	}
	h.clearSessionCookie(c)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func PrincipalFromContext(c echo.Context) (Principal, bool) {
	principal, ok := c.Get("auth.principal").(Principal)
	return principal, ok
}

func (h *Handler) setSessionCookie(c echo.Context, token string) {
	ttl := h.service.Tokens().TTL()
	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(ttl).UTC(),
		MaxAge:   int(ttl / time.Second),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func decodeJSON(c echo.Context, destination any) error {
	body := http.MaxBytesReader(c.Response(), c.Request().Body, maxAuthBodyBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return apperror.New(http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "请求内容过大")
		}
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "请求JSON格式错误")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "请求只能包含一个JSON对象")
	}
	return nil
}

func serviceError(err error) error {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return apperror.Validation([]apperror.FieldError{{
			Field:   validation.Violation.Field,
			Message: validation.Violation.Message,
		}})
	}
	switch {
	case errors.Is(err, ErrAlreadyInitialized):
		return apperror.New(http.StatusConflict, "ALREADY_INITIALIZED", "学校已完成初始化")
	case errors.Is(err, ErrInvalidSchoolCode):
		return apperror.New(http.StatusBadRequest, "INVALID_SCHOOL_CODE", "学校代码无效")
	case errors.Is(err, ErrEmailAlreadyExists):
		return apperror.New(http.StatusConflict, "EMAIL_ALREADY_EXISTS", "该邮箱在本学校已注册")
	case errors.Is(err, ErrInvalidCredentials):
		return apperror.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "认证信息无效")
	case errors.Is(err, ErrOldPassword):
		return apperror.New(http.StatusUnauthorized, "INVALID_OLD_PASSWORD", "原密码不正确")
	case errors.Is(err, ErrSessionInvalid):
		return apperror.New(http.StatusUnauthorized, "SESSION_INVALID", "会话已失效，请重新登录")
	default:
		return apperror.Internal(err)
	}
}
