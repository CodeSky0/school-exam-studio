package ai

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	ConversationStatusActive   = "active"
	ConversationStatusArchived = "archived"

	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
	MessageRoleTool      = "tool"
	MessageRoleSystem    = "system"

	MessageStatusRunning   = "running"
	MessageStatusSucceeded = "succeeded"
	MessageStatusFailed    = "failed"

	ToolStatusRunning   = "running"
	ToolStatusSucceeded = "succeeded"
	ToolStatusFailed    = "failed"
	ToolStatusSkipped   = "skipped"

	AttachmentKindImage = "image"
	AttachmentKindPDF   = "pdf"
	AttachmentKindDocx  = "docx"
	AttachmentKindText  = "text"

	AttachmentStatusUploaded = "uploaded"
	AttachmentStatusParsed   = "parsed"
	AttachmentStatusFailed   = "failed"
)

// Conversation 是对话列表使用的会话摘要。
type Conversation struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	Pinned        bool      `json:"pinned"`
	CreatorID     string    `json:"creator_id"`
	MessageCount  int       `json:"message_count"`
	LastMessageAt time.Time `json:"last_message_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ToolCall 是一次工具调用的完整记录，前端据此渲染工具时间线。
type ToolCall struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Label      string         `json:"label"`
	Status     string         `json:"status"`
	Summary    string         `json:"summary"`
	Error      string         `json:"error,omitempty"`
	Arguments  map[string]any `json:"arguments"`
	Result     map[string]any `json:"result,omitempty"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

// AttachmentRef 是消息中展示的附件摘要，不包含磁盘路径与全文。
type AttachmentRef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MIME        string `json:"mime"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	TextPreview string `json:"text_preview"`
	SizeBytes   int64  `json:"size_bytes"`
	TextChars   int    `json:"text_chars"`
	Error       string `json:"error,omitempty"`
}

type ChatMessage struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversation_id"`
	Role           string          `json:"role"`
	Content        string          `json:"content"`
	Status         string          `json:"status"`
	Provider       string          `json:"provider"`
	Model          string          `json:"model"`
	ToolCalls      []ToolCall      `json:"tool_calls"`
	Attachments    []AttachmentRef `json:"attachments"`
	RunID          string          `json:"run_id,omitempty"`
	Error          string          `json:"error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Attachment 是附件的内部完整记录，StoragePath 与 ExtractedText 不直接返回前端。
type Attachment struct {
	ID             string
	CreatorID      string
	ConversationID string
	Name           string
	MIME           string
	Kind           string
	SizeBytes      int64
	StoragePath    string
	SHA256         string
	ExtractedText  string
	TextChars      int
	Status         string
	Error          string
	CreatedAt      time.Time
}

func (a Attachment) Ref() AttachmentRef {
	return AttachmentRef{
		ID:          a.ID,
		Name:        a.Name,
		MIME:        a.MIME,
		Kind:        a.Kind,
		Status:      a.Status,
		TextPreview: textPreview(a.ExtractedText, 180),
		SizeBytes:   a.SizeBytes,
		TextChars:   a.TextChars,
		Error:       a.Error,
	}
}

type SendMessageInput struct {
	Content       string   `json:"content"`
	AttachmentIDs []string `json:"attachment_ids"`
	Model         string   `json:"model,omitempty"`
}

type SendMessageResult struct {
	Conversation       Conversation    `json:"conversation"`
	UserMessage        ChatMessage     `json:"user_message"`
	AssistantMessage   ChatMessage     `json:"assistant_message"`
	AttachmentDelivery []AttachmentRef `json:"attachments,omitempty"`
}

func textPreview(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func toolCallsToMaps(calls []ToolCall) []map[string]any {
	if len(calls) == 0 {
		return []map[string]any{}
	}
	raw, err := json.Marshal(calls)
	if err != nil {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(calls))
	if err := json.Unmarshal(raw, &result); err != nil {
		return []map[string]any{}
	}
	return result
}

func mapsToToolCalls(values []map[string]any) []ToolCall {
	if len(values) == 0 {
		return []ToolCall{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return []ToolCall{}
	}
	result := make([]ToolCall, 0, len(values))
	if err := json.Unmarshal(raw, &result); err != nil {
		return []ToolCall{}
	}
	return result
}

func attachmentsToMaps(refs []AttachmentRef) []map[string]any {
	if len(refs) == 0 {
		return []map[string]any{}
	}
	raw, err := json.Marshal(refs)
	if err != nil {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(refs))
	if err := json.Unmarshal(raw, &result); err != nil {
		return []map[string]any{}
	}
	return result
}

func mapsToAttachments(values []map[string]any) []AttachmentRef {
	if len(values) == 0 {
		return []AttachmentRef{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return []AttachmentRef{}
	}
	result := make([]AttachmentRef, 0, len(values))
	if err := json.Unmarshal(raw, &result); err != nil {
		return []AttachmentRef{}
	}
	return result
}
