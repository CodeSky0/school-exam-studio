package paper

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

const maxPaperBodyBytes = 1 << 20

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *echo.Group) {
	group.GET("/papers", h.List)
	group.POST("/papers", h.Create)
	group.GET("/papers/:id", h.Get)
	group.PATCH("/papers/:id", h.Update)
	group.DELETE("/papers/:id", h.Delete)
}

func (h *Handler) Create(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return paperUnauthorized()
	}
	var input Input
	if err := decodePaperJSON(c, &input); err != nil {
		return err
	}
	created, err := h.service.Create(c.Request().Context(), principal, input)
	if err != nil {
		return paperError(err)
	}
	return c.JSON(http.StatusCreated, created)
}

func (h *Handler) Get(c echo.Context) error {
	principal, id, err := paperPrincipalAndID(c)
	if err != nil {
		return err
	}
	result, err := h.service.Get(c.Request().Context(), principal, id)
	if err != nil {
		return paperError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) List(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return paperUnauthorized()
	}
	filter, err := parsePaperFilter(c)
	if err != nil {
		return err
	}
	result, err := h.service.List(c.Request().Context(), principal, filter)
	if err != nil {
		return paperError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Update(c echo.Context) error {
	principal, id, err := paperPrincipalAndID(c)
	if err != nil {
		return err
	}
	var input Input
	if err := decodePaperJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.Update(c.Request().Context(), principal, id, input)
	if err != nil {
		return paperError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Delete(c echo.Context) error {
	principal, id, err := paperPrincipalAndID(c)
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), principal, id); err != nil {
		return paperError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func paperPrincipalAndID(c echo.Context) (platformmiddleware.Principal, uuid.UUID, error) {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return platformmiddleware.Principal{}, uuid.Nil, paperUnauthorized()
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return platformmiddleware.Principal{}, uuid.Nil, apperror.New(http.StatusBadRequest, "INVALID_PAPER_ID", "试卷ID格式不正确")
	}
	return principal, id, nil
}

func parsePaperFilter(c echo.Context) (Filter, error) {
	query := c.QueryParams()
	filter := Filter{Subject: query.Get("subject"), Grade: query.Get("grade"), Page: 1, PageSize: 20}
	var err error
	if filter.Page, err = paperQueryPositiveInt(query.Get("page"), 1, "page"); err != nil {
		return Filter{}, err
	}
	if filter.PageSize, err = paperQueryPositiveInt(query.Get("page_size"), 20, "page_size"); err != nil {
		return Filter{}, err
	}
	return filter, nil
}

func paperQueryPositiveInt(value string, fallback int, field string) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, apperror.Validation([]apperror.FieldError{{Field: field, Message: "分页参数必须为正整数"}})
	}
	return parsed, nil
}

func decodePaperJSON(c echo.Context, destination any) error {
	body := http.MaxBytesReader(c.Response(), c.Request().Body, maxPaperBodyBytes)
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

func paperError(err error) error {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return apperror.Validation([]apperror.FieldError{{
			Field:   validation.Violation.Field,
			Message: validation.Violation.Message,
		}})
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return apperror.New(http.StatusNotFound, "PAPER_NOT_FOUND", "试卷不存在")
	case errors.Is(err, ErrForbidden):
		return apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	case errors.Is(err, ErrDuplicateQuestion):
		return apperror.New(http.StatusUnprocessableEntity, "DUPLICATE_QUESTION", "试卷题目不得重复")
	case errors.Is(err, ErrQuestionMismatch):
		return apperror.New(http.StatusUnprocessableEntity, "QUESTION_MISMATCH", "题目学科或年级与试卷不一致")
	case errors.Is(err, ErrQuestionUnavailable):
		return apperror.New(http.StatusUnprocessableEntity, "QUESTION_UNAVAILABLE", "试卷包含不存在、跨学校或未发布的题目")
	default:
		return apperror.Internal(err)
	}
}

func paperUnauthorized() error {
	return apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
}
