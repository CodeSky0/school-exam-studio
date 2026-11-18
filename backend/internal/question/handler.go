package question

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
)

const maxQuestionBodyBytes = 1 << 20

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *echo.Group) {
	group.GET("/questions", h.List)
	group.POST("/questions", h.Create)
	group.POST("/questions/resolve", h.Resolve)
	group.GET("/questions/:id", h.Get)
	group.PATCH("/questions/:id", h.Update)
	group.DELETE("/questions/:id", h.Delete)
	group.PATCH("/admin/questions/:id/status", h.UpdateStatus)
	group.PATCH("/admin/questions/batch/status", h.BatchUpdateStatus)
	group.POST("/questions/batch/delete", h.BatchDelete)
}

func (h *Handler) Create(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var input Input
	if err := decodeQuestionJSON(c, &input); err != nil {
		return err
	}
	created, err := h.service.Create(c.Request().Context(), principal, input)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusCreated, created)
}

func (h *Handler) Get(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	result, err := h.service.Get(c.Request().Context(), principal, id)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) List(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	filter, err := parseFilter(c)
	if err != nil {
		return err
	}
	result, err := h.service.List(c.Request().Context(), principal, filter)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Update(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	var input Input
	if err := decodeQuestionJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.Update(c.Request().Context(), principal, id, input)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Delete(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), principal, id); err != nil {
		return questionError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) Resolve(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var request struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := decodeQuestionJSON(c, &request); err != nil {
		return err
	}
	items, err := h.service.ResolvePublished(c.Request().Context(), principal, request.IDs)
	if err != nil {
		return questionError(err)
	}
	if items == nil {
		items = []Question{}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) UpdateStatus(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	var request struct {
		Status string `json:"status"`
	}
	if err := decodeQuestionJSON(c, &request); err != nil {
		return err
	}
	result, err := h.service.UpdateStatus(c.Request().Context(), principal, id, request.Status)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) BatchUpdateStatus(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var request struct {
		IDs    []uuid.UUID `json:"ids"`
		Status string      `json:"status"`
	}
	if err := decodeQuestionJSON(c, &request); err != nil {
		return err
	}
	result, err := h.service.BatchUpdateStatus(c.Request().Context(), principal, request.IDs, request.Status)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) BatchDelete(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var request struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := decodeQuestionJSON(c, &request); err != nil {
		return err
	}
	result, err := h.service.BatchDelete(c.Request().Context(), principal, request.IDs)
	if err != nil {
		return questionError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func principalAndID(c echo.Context) (platformmiddleware.Principal, uuid.UUID, error) {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return platformmiddleware.Principal{}, uuid.Nil, unauthorized()
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return platformmiddleware.Principal{}, uuid.Nil, apperror.New(http.StatusBadRequest, "INVALID_QUESTION_ID", "题目ID格式不正确")
	}
	return principal, id, nil
}

func parseFilter(c echo.Context) (QuestionFilter, error) {
	query := c.QueryParams()
	filter := QuestionFilter{
		Subject:   query.Get("subject"),
		Grade:     query.Get("grade"),
		Type:      query.Get("question_type"),
		Tag:       query.Get("tag"),
		CreatorID: query.Get("creator_id"),
		Status:    query.Get("status"),
		Page:      1,
		PageSize:  20,
	}
	var err error
	if filter.Page, err = queryPositiveInt(query.Get("page"), 1, "page"); err != nil {
		return QuestionFilter{}, err
	}
	if filter.PageSize, err = queryPositiveInt(query.Get("page_size"), 20, "page_size"); err != nil {
		return QuestionFilter{}, err
	}
	if value := query.Get("difficulty"); value != "" {
		filter.Difficulty, err = strconv.Atoi(value)
		if err != nil || filter.Difficulty < 1 || filter.Difficulty > 5 {
			return QuestionFilter{}, apperror.Validation([]apperror.FieldError{{Field: "difficulty", Message: "难度筛选值必须为1-5"}})
		}
	}
	if filter.Type != "" && !ValidType(filter.Type) {
		return QuestionFilter{}, apperror.Validation([]apperror.FieldError{{Field: "question_type", Message: "题型筛选值不受支持"}})
	}
	if filter.Status != "" && !ValidStatus(filter.Status) {
		return QuestionFilter{}, apperror.Validation([]apperror.FieldError{{Field: "status", Message: "题目状态筛选值不受支持"}})
	}
	if filter.CreatorID != "" {
		if _, err := uuid.Parse(filter.CreatorID); err != nil {
			return QuestionFilter{}, apperror.Validation([]apperror.FieldError{{Field: "creator_id", Message: "创建人ID格式不正确"}})
		}
	}
	return filter, nil
}

func queryPositiveInt(value string, fallback int, field string) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, apperror.Validation([]apperror.FieldError{{Field: field, Message: "分页参数必须为正整数"}})
	}
	return parsed, nil
}

func decodeQuestionJSON(c echo.Context, destination any) error {
	body := http.MaxBytesReader(c.Response(), c.Request().Body, maxQuestionBodyBytes)
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

func questionError(err error) error {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return apperror.Validation([]apperror.FieldError{{
			Field:   validation.Violation.Field,
			Message: validation.Violation.Message,
		}})
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return apperror.New(http.StatusNotFound, "QUESTION_NOT_FOUND", "题目不存在")
	case errors.Is(err, ErrSubjectMismatch):
		return apperror.New(http.StatusForbidden, "SUBJECT_NOT_ALLOWED", "教师只能使用本人账户学科")
	case errors.Is(err, ErrForbidden):
		return apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	case errors.Is(err, ErrInvalidTransition):
		return apperror.New(http.StatusConflict, "INVALID_STATUS_TRANSITION", "题目状态不允许该变更")
	default:
		return apperror.Internal(err)
	}
}

func unauthorized() error {
	return apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
}
