package importer

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/apperror"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
)

const TemplateCSV = `题型,年级,学科,难度,知识点标签,题干,选项A,选项B,选项C,选项D,答案,解析
单选题,七年级,数学,2,代数;方程,求方程 $x^2=4$ 的正根,0,2,-2,4,B,正数的平方根取正值
`

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *echo.Group) {
	group.POST("/questions/import/validate", h.Validate)
	group.POST("/questions/import", h.Import)
	group.GET("/questions/import/template", h.Template)
}

func (h *Handler) Validate(c echo.Context) error {
	reader, closeFile, err := uploadedCSV(c)
	if err != nil {
		return err
	}
	defer closeFile()
	result, err := h.service.Validate(c.Request().Context(), mustPrincipal(c), reader)
	if err != nil {
		return importError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Import(c echo.Context) error {
	reader, closeFile, err := uploadedCSV(c)
	if err != nil {
		return err
	}
	defer closeFile()
	result, err := h.service.ImportCSV(c.Request().Context(), mustPrincipal(c), reader)
	if err != nil {
		return importError(err)
	}
	if len(result.Errors) > 0 {
		return c.JSON(http.StatusUnprocessableEntity, result)
	}
	return c.JSON(http.StatusOK, map[string]int{"imported": result.ValidCount})
}

func (h *Handler) Template(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="questions-template.csv"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", []byte(TemplateCSV))
}

func uploadedCSV(c echo.Context) (io.Reader, func(), error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, func() {}, apperror.New(http.StatusBadRequest, "INVALID_CSV_UPLOAD", "请在multipart字段file中上传CSV文件")
	}
	if fileHeader.Size > MaxFileSize {
		return nil, func() {}, apperror.New(http.StatusRequestEntityTooLarge, "CSV_TOO_LARGE", "CSV文件不能超过5MB")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, func() {}, apperror.New(http.StatusBadRequest, "INVALID_CSV_UPLOAD", "无法读取上传的CSV文件")
	}
	return file, func() { _ = file.Close() }, nil
}

func mustPrincipal(c echo.Context) auth.Principal {
	principal, _ := platformmiddleware.PrincipalFrom(c)
	return principal
}

func importError(err error) error {
	switch {
	case errors.Is(err, ErrFileTooLarge):
		return apperror.New(http.StatusRequestEntityTooLarge, "CSV_TOO_LARGE", "CSV文件不能超过5MB")
	case errors.Is(err, ErrForbidden):
		return apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	default:
		return apperror.Internal(fmt.Errorf("CSV import: %w", err))
	}
}
