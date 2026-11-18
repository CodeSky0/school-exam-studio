package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"school-exam-studio/backend/internal/auth"
	platformmiddleware "school-exam-studio/backend/internal/platform/middleware"
)

func (h *Handler) ListConversations(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	limit := 30
	if value := c.QueryParam("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			return echo.NewHTTPError(http.StatusBadRequest, "limit 必须为1-100")
		}
		limit = parsed
	}
	items, err := h.service.ListConversations(c.Request().Context(), principal, limit)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateConversation(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var input struct {
		Title string `json:"title"`
	}
	if err := decodeOptionalAIJSON(c, &input); err != nil {
		return err
	}
	item, err := h.service.CreateConversation(c.Request().Context(), principal, input.Title)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) GetConversation(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	conversation, messages, err := h.service.GetConversation(c.Request().Context(), principal, id)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"conversation": conversation, "messages": messages})
}

func (h *Handler) UpdateConversation(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	var input struct {
		Title  *string `json:"title"`
		Pinned *bool   `json:"pinned"`
	}
	if err := decodeOptionalAIJSON(c, &input); err != nil {
		return err
	}
	if input.Title == nil && input.Pinned == nil {
		return apperrorValidation("title", "至少需要提供标题或置顶状态")
	}
	existingConversation, _, err := h.service.GetConversation(c.Request().Context(), principal, id)
	if err != nil {
		return aiError(err)
	}
	title := existingConversation.Title
	pinned := existingConversation.Pinned
	if input.Title != nil {
		title = *input.Title
	}
	if input.Pinned != nil {
		pinned = *input.Pinned
	}
	conversation, err := h.service.UpdateConversation(c.Request().Context(), principal, id, title, pinned)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, conversation)
}

func (h *Handler) DeleteConversation(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	if err := h.service.DeleteConversation(c.Request().Context(), principal, id); err != nil {
		return aiError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) BatchDeleteConversations(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	var input struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := decodeAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.BatchDeleteConversations(c.Request().Context(), principal, input.IDs)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) ListConversationMessages(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	limit := 100
	if value := c.QueryParam("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 200 {
			return apperrorValidation("limit", "limit 必须为1-200")
		}
		limit = parsed
	}
	items, err := h.service.ListMessages(c.Request().Context(), principal, id, limit)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) SendMessage(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	var input SendMessageInput
	if err := decodeAIJSON(c, &input); err != nil {
		return err
	}
	result, err := h.service.SendMessage(c.Request().Context(), principal, id, input)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, result)
}

// StreamMessage 以 SSE 推送用户消息、工具调用与最终回复，供对话工作台实时渲染。
func (h *Handler) StreamMessage(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	var input SendMessageInput
	if err := decodeAIJSON(c, &input); err != nil {
		return err
	}
	response := c.Response()
	response.Header().Set(echo.HeaderContentType, "text/event-stream; charset=utf-8")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(http.StatusOK)
	flusher, _ := response.Writer.(http.Flusher)
	sink := func(event ChatStreamEvent) {
		_ = writeChatEvent(response, flusher, event)
	}
	result, err := h.service.SendMessageStream(c.Request().Context(), principal, id, input, sink)
	if err != nil {
		sink(ChatStreamEvent{Type: ChatEventError, Error: streamAIErrorMessage(err)})
		return nil
	}
	sink(ChatStreamEvent{
		Type:         ChatEventAssistantMessage,
		Message:      &result.AssistantMessage,
		Conversation: &result.Conversation,
	})
	sink(ChatStreamEvent{Type: ChatEventDone})
	return nil
}

func writeChatEvent(writer io.Writer, flusher http.Flusher, event ChatStreamEvent) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event.Type, raw); err != nil {
		return err
	}
	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

func streamAIErrorMessage(err error) string {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return validation.Violation.Message
	}
	switch {
	case errors.Is(err, ErrForbidden):
		return "没有操作权限"
	case errors.Is(err, ErrNotFound):
		return "会话不存在或已被删除"
	default:
		return "AI 处理失败，请稍后重试"
	}
}

func (h *Handler) UploadAttachment(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	if err := c.Request().ParseMultipartForm(26 << 20); err != nil {
		return apperrorValidation("file", "无法读取上传内容")
	}
	file, header, err := c.Request().FormFile("file")
	if err != nil {
		return apperrorValidation("file", "请选择要上传的文件")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxDocBytes+1))
	if err != nil {
		return apperrorValidation("file", "无法读取上传文件")
	}
	if len(data) > MaxDocBytes {
		return apperrorValidation("file", "文件超过 20 MiB 限制")
	}
	attachment, err := h.service.UploadAttachment(
		c.Request().Context(), principal, c.FormValue("conversation_id"), header.Filename,
		header.Header.Get("Content-Type"), data,
	)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusCreated, attachment.Ref())
}

func (h *Handler) GetAttachment(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	attachment, err := h.service.GetAttachment(c.Request().Context(), principal, id)
	if err != nil {
		return aiError(err)
	}
	return c.JSON(http.StatusOK, attachment.Ref())
}

func (h *Handler) DownloadAttachment(c echo.Context) error {
	principal, id, err := principalAndID(c)
	if err != nil {
		return err
	}
	attachment, file, err := h.service.OpenAttachment(c.Request().Context(), principal, id)
	if err != nil {
		return aiError(err)
	}
	defer file.Close()
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	c.Response().Header().Set("Cache-Control", "private, max-age=300")
	c.Response().Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(attachment.Name))
	return c.Stream(http.StatusOK, attachment.MIME, file)
}

func (h *Handler) ListTools(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	return c.JSON(http.StatusOK, map[string]any{"items": h.service.ToolCatalog(principal)})
}

func (h *Handler) ListSkills(c echo.Context) error {
	principal, ok := platformmiddleware.PrincipalFrom(c)
	if !ok {
		return unauthorized()
	}
	if principal.Role != auth.RoleAdmin && principal.Role != auth.RoleTeacher {
		return aiError(ErrForbidden)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": h.service.ListSkills()})
}

func apperrorValidation(field, message string) error {
	return aiError(validationError(field, message))
}
