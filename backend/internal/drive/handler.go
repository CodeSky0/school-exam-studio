package drive

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
)

const maxDriveJSONBytes = 64 << 10

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *echo.Group) {
	group.GET("/cloud/items", h.List)
	group.POST("/cloud/folders", h.CreateFolder)
	group.POST("/cloud/files", h.Upload)
	group.GET("/cloud/items/:id/download", h.Download)
	group.DELETE("/cloud/items/:id", h.Delete)
}

func (h *Handler) List(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	result, err := h.service.List(c.Request().Context(), principal, c.QueryParam("parent_id"))
	if err != nil {
		return driveError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateFolder(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var input CreateFolderInput
	if err := decodeDriveJSON(c, &input); err != nil {
		return err
	}
	item, err := h.service.CreateFolder(c.Request().Context(), principal, input)
	if err != nil {
		return driveError(err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) Upload(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, MaxFileBytes+(1<<20))
	if err := c.Request().ParseMultipartForm(MaxFileBytes + (1 << 20)); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return driveError(validationError("file", "文件超过 50 MiB 限制"))
		}
		return driveError(validationError("file", "无法读取上传内容"))
	}
	file, header, err := c.Request().FormFile("file")
	if err != nil {
		return driveError(validationError("file", "请选择要上传的文件"))
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return driveError(validationError("file", "无法读取上传文件"))
	}
	if len(data) > MaxFileBytes {
		return driveError(validationError("file", "文件超过 50 MiB 限制"))
	}
	item, err := h.service.Upload(c.Request().Context(), principal, c.FormValue("parent_id"), header.Filename, data)
	if err != nil {
		return driveError(err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) Download(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return driveError(validationError("id", "文件ID格式不正确"))
	}
	item, file, err := h.service.OpenFile(c.Request().Context(), principal, id)
	if err != nil {
		return driveError(err)
	}
	defer file.Close()
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	c.Response().Header().Set("Cache-Control", "private, no-store")
	c.Response().Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(item.Name))
	return c.Stream(http.StatusOK, item.MIME, file)
}

func (h *Handler) Delete(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return driveError(validationError("id", "文件ID格式不正确"))
	}
	deleted, err := h.service.Delete(c.Request().Context(), principal, id)
	if err != nil {
		return driveError(err)
	}
	return c.JSON(http.StatusOK, map[string]int{"deleted": deleted})
}

func decodeDriveJSON(c echo.Context, destination any) error {
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxDriveJSONBytes))
	if err != nil {
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "无法读取请求内容")
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "请求体不能为空")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "请求JSON格式错误或包含不接受字段")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "请求只能包含一个JSON对象")
	}
	return nil
}

func driveError(err error) error {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return apperror.Validation([]apperror.FieldError{{
			Field:   validation.Field,
			Message: validation.Message,
		}})
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return apperror.New(http.StatusNotFound, "CLOUD_ITEM_NOT_FOUND", "文件或文件夹不存在")
	case errors.Is(err, ErrForbidden):
		return apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	case errors.Is(err, ErrConflict):
		return apperror.New(http.StatusConflict, "CLOUD_NAME_CONFLICT", "同名文件夹已存在")
	default:
		return apperror.Internal(err)
	}
}

func unauthorized() error {
	return apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
}
