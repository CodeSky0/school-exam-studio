package ai

import (
	"bytes"
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
	"school-exam-studio/backend/internal/question"
)

const maxAIBodyBytes = 1 << 20

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(group *echo.Group) {
	group.GET("/ai/status", h.Status)
	group.GET("/admin/ai/settings", h.GetSchoolAISettings)
	group.PATCH("/admin/ai/settings", h.UpdateSchoolAISettings)
	group.POST("/admin/ai/test", h.TestConnection)
	group.POST("/ai/analyze", h.Analyze)
	group.POST("/ai/questions/:id/analyze", h.AnalyzeQuestion)
	group.POST("/ai/questions/:id/variants", h.Variants)
	group.POST("/ai/generate", h.Generate)
	group.GET("/ai/bank/audit", h.AuditBank)
	group.POST("/ai/agent/runs", h.CreateRun)
	group.GET("/ai/agent/runs", h.ListRuns)
	group.GET("/ai/agent/runs/:id", h.GetRun)
	group.GET("/ai/audit-logs", h.AuditLogs)

	group.GET("/ai/conversations", h.ListConversations)
	group.POST("/ai/conversations", h.CreateConversation)
	group.POST("/ai/conversations/batch/delete", h.BatchDeleteConversations)
	group.GET("/ai/conversations/:id", h.GetConversation)
	group.PATCH("/ai/conversations/:id", h.UpdateConversation)
	group.DELETE("/ai/conversations/:id", h.DeleteConversation)
	group.GET("/ai/conversations/:id/messages", h.ListConversationMessages)
	group.POST("/ai/conversations/:id/messages", h.SendMessage)
	group.POST("/ai/conversations/:id/messages/stream", h.StreamMessage)
	group.POST("/ai/attachments", h.UploadAttachment)
	group.POST("/ai/attachments/cloud", h.AttachCloudFiles)
	group.GET("/ai/attachments/:id", h.GetAttachment)
	group.GET("/ai/attachments/:id/content", h.DownloadAttachment)
	group.GET("/ai/tools", h.ListTools)
	group.POST("/ai/tools/:name", h.RunTool)
	group.GET("/ai/skills", h.ListSkills)
	group.POST("/ai/skills/:id/run", h.RunSkill)
}

func (h *Handler) Status(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	return c.JSON(http.StatusOK, h.service.Status(c.Request().Context(), principal))
}

// RunTool 允许前端与外部 Agent 直接执行工具，复用统一校验、权限与审计。
func (h *Handler) RunTool(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	args := map[string]any{}
	if err := decodeOptionalAIJSON(c, &args); err != nil {
		return err
	}
	result, err := h.service.RunTool(c.Request().Context(), principal, ToolContext{RunID: uuid.NewString()}, c.Param("name"), args)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

// RunSkill 以技能提示词发起一次会话消息，供前端一键执行与外部 Agent 调用。
func (h *Handler) RunSkill(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var input SkillRunInput
	if err := decodeOptionalAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.RunSkill(c.Request().Context(), principal, c.Param("id"), input)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) GetSchoolAISettings(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	if principal.Role != auth.RoleAdmin {
		return aiError(ErrForbidden)
	}
	result, err := h.service.GetSchoolAISettings(c.Request().Context(), principal)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) UpdateSchoolAISettings(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	if principal.Role != auth.RoleAdmin {
		return aiError(ErrForbidden)
	}
	var input SchoolAISettingsInput
	if err := decodeAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.UpdateSchoolAISettings(c.Request().Context(), principal, input)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) TestConnection(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	if principal.Role != auth.RoleAdmin {
		return aiError(ErrForbidden)
	}
	var input ConnectionTestInput
	if err := decodeOptionalAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.TestConnection(c.Request().Context(), principal, input.Model)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Analyze(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var input AnalyzeInput
	if err := decodeAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.Analyze(c.Request().Context(), principal, input)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) AnalyzeQuestion(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	var request struct {
		Apply bool `json:"apply"`
	}
	if err := decodeOptionalAIJSON(c, &request); err != nil {
		return err
	}
	result, err := h.service.AnalyzeQuestion(c.Request().Context(), principal, id, request.Apply)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Variants(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	var input VariantInput
	if err := decodeAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.GenerateVariants(c.Request().Context(), principal, id, input)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Generate(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var input GenerateInput
	if err := decodeAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.Generate(c.Request().Context(), principal, input)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) AuditBank(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	limit := 500
	if value := c.QueryParam("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 1000 {
			return apperror.Validation([]apperror.FieldError{{Field: "limit", Message: "limit 必须为1-1000"}})
		}
		limit = parsed
	}
	result, err := h.service.AuditBank(c.Request().Context(), principal, limit)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateRun(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var input AgentRequest
	if err := decodeAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.ExecuteAgent(c.Request().Context(), principal, input)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) ListRuns(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	limit := 20
	if value := c.QueryParam("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			return apperror.Validation([]apperror.FieldError{{Field: "limit", Message: "limit 必须为1-100"}})
		}
		limit = parsed
	}
	items, err := h.service.ListRuns(c.Request().Context(), principal, limit)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetRun(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	result, err := h.service.GetRun(c.Request().Context(), principal, id)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) AuditLogs(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	limit := 50
	if value := c.QueryParam("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 200 {
			return apperror.Validation([]apperror.FieldError{{Field: "limit", Message: "limit 必须为1-200"}})
		}
		limit = parsed
	}
	items, err := h.service.ListAudits(c.Request().Context(), principal, limit)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func principalAndID(c echo.Context) (platformmiddleware.Principal, uuid.UUID, error) {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return platformmiddleware.Principal{}, uuid.Nil, unauthorized()
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return platformmiddleware.Principal{}, uuid.Nil, apperror.New(http.StatusBadRequest, "INVALID_ID", "ID格式不正确")
	}
	return principal, id, nil
}

func decodeAIJSON(c echo.Context, destination any) error {
	body, err := readAIBody(c)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return apperror.New(http.StatusBadRequest, "INVALID_JSON", "请求体不能为空")
	}
	return unmarshalStrict(body, destination)
}

func decodeOptionalAIJSON(c echo.Context, destination any) error {
	body, err := readAIBody(c)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return unmarshalStrict(body, destination)
}

func readAIBody(c echo.Context) ([]byte, error) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Response(), c.Request().Body, maxAIBodyBytes))
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return nil, apperror.New(http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "请求内容过大")
		}
		return nil, apperror.New(http.StatusBadRequest, "INVALID_JSON", "无法读取请求内容")
	}
	return body, nil
}

func unmarshalStrict(body []byte, destination any) error {
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

func aiError(err error) error {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return apperror.Validation([]apperror.FieldError{{
			Field:   validation.Violation.Field,
			Message: validation.Violation.Message,
		}})
	}
	var questionValidation *question.ValidationError
	if errors.As(err, &questionValidation) {
		return apperror.Validation([]apperror.FieldError{{
			Field:   questionValidation.Violation.Field,
			Message: questionValidation.Violation.Message,
		}})
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return apperror.New(http.StatusNotFound, "AI_NOT_FOUND", "AI 记录不存在")
	case errors.Is(err, ErrForbidden):
		return apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	case errors.Is(err, question.ErrNotFound):
		return apperror.New(http.StatusNotFound, "QUESTION_NOT_FOUND", "题目不存在")
	case errors.Is(err, question.ErrForbidden), errors.Is(err, question.ErrSubjectMismatch):
		return apperror.New(http.StatusForbidden, "FORBIDDEN", "没有操作权限")
	default:
		return apperror.Internal(err)
	}
}

func unauthorized() error {
	return apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "未认证或会话已失效")
}
