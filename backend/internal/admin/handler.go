package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
)

const maxAdminBodyBytes = 64 << 10

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *echo.Group) {
	group.GET("/admin/teachers", h.ListTeachers)
	group.PATCH("/admin/teachers/:id/status", h.UpdateTeacherStatus)
	group.POST("/admin/teachers/:id/reset-password", h.ResetTeacherPassword)
	group.GET("/admin/school", h.GetSchool)
	group.PATCH("/admin/school", h.UpdateSchool)
	group.GET("/admin/stats", h.Stats)
	group.GET("/teacher/stats", h.TeacherStats)
}

func (h *Handler) ListTeachers(c echo.Context) error {
	principal, err := principalWithRole(c, auth.RoleAdmin)
	if err != nil {
		return err
	}
	filter := TeacherFilter{
		Subject: c.QueryParam("subject"),
		Status:  c.QueryParam("status"),
		Page:    1,
		PageSize: 20,
	}
	if filter.Page, err = positiveQueryInt(c.QueryParam("page"), 1, "page"); err != nil {
		return err
	}
	if filter.PageSize, err = positiveQueryInt(c.QueryParam("page_size"), 20, "page_size"); err != nil {
		return err
	}
	result, err := h.service.ListTeachers(c.Request().Context(), principal, filter)
	if err != nil {
		return adminError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) UpdateTeacherStatus(c echo.Context) error {
	principal, teacherID, err := adminPrincipalAndTeacherID(c)
	if err != nil {
		return err
	}
	var request struct {
		Status string `json:"status"`
	}
	if err := decodeAdminJSON(c, &request); err != nil {
		return err
	}
	result, err := h.service.UpdateTeacherStatus(c.Request().Context(), principal, teacherID, request.Status)
	if err != nil {
		return adminError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) ResetTeacherPassword(c echo.Context) error {
	principal, teacherID, err := adminPrincipalAndTeacherID(c)
	if err != nil {
		return err
	}
	temporaryPassword, err := h.service.ResetTeacherPassword(c.Request().Context(), principal, teacherID)
	if err != nil {
		return adminError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"temporary_password": temporaryPassword})
}

func (h *Handler) GetSchool(c echo.Context) error {
	principal, err := principalWithRole(c, auth.RoleAdmin)
	if err != nil {
		return err
	}
	result, err := h.service.GetSchool(c.Request().Context(), principal)
	if err != nil {
		return adminError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) UpdateSchool(c echo.Context) error {
	principal, err := principalWithRole(c, auth.RoleAdmin)
	if err != nil {
		return err
	}
	var request struct {
		Name string `json:"name"`
	}
	if err := decodeAdminJSON(c, &request); err != nil {
		return err
	}
	result, err := h.service.UpdateSchool(c.Request().Context(), principal, request.Name)
	if err != nil {
		return adminError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Stats(c echo.Context) error {
	principal, err := principalWithRole(c, auth.RoleAdmin)
	if err != nil {
		return err
	}
	result, err := h.service.Stats(c.Request().Context(), principal)
	if err != nil {
		return adminError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) TeacherStats(c echo.Context) error {
	principal, err := principalWithRole(c, auth.RoleTeacher)
	if err != nil {
		return err
	}
	result, err := h.service.TeacherStats(c.Request().Context(), principal)
	if err != nil {
		return adminError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func adminPrincipalAndTeacherID(c echo.Context) (auth.Principal, uuid.UUID, error) {
	principal, err := principalWithRole(c, auth.RoleAdmin)
	if err != nil {
		return auth.Principal{}, uuid.Nil, err
	}
	teacherID, parseErr := uuid.Parse(c.Param("id"))
	if parseErr != nil {
		return auth.Principal{}, uuid.Nil, apperror.New(http.StatusBadRequest, "INVALID_TEACHER_ID", "教师ID格式不正确")
	}
	return principal, teacherID, nil
}

func principalWithRole(c echo.Context, role string) (auth.Principal, error) {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return auth.Principal{}, apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
	}
	if principal.Role != role {
		return auth.Principal{}, apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	}
	return principal, nil
}

func positiveQueryInt(value string, fallback int, field string) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, apperror.Validation([]apperror.FieldError{{Field: field, Message: "分页参数必须为正整数"}})
	}
	return parsed, nil
}

func decodeAdminJSON(c echo.Context, destination any) error {
	body := http.MaxBytesReader(c.Response(), c.Request().Body, maxAdminBodyBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return apperror.New(http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "请求内容过大")
		}
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "请求JSON格式错误或包含不接受字段")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "请求只能包含一个JSON对象")
	}
	return nil
}

func adminError(err error) error {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return apperror.Validation([]apperror.FieldError{{
			Field:   validation.Violation.Field,
			Message: validation.Violation.Message,
		}})
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return apperror.New(http.StatusNotFound, "ADMIN_RESOURCE_NOT_FOUND", "管理资源不存在")
	case errors.Is(err, ErrForbidden):
		return apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	default:
		return apperror.Internal(err)
	}
}
