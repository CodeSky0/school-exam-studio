package ai

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteChatEventFramesSSE(t *testing.T) {
	recorder := httptest.NewRecorder()
	call := ToolCall{ID: "call-1", Name: "search_questions", Label: "检索题目", Status: ToolStatusRunning}
	if err := writeChatEvent(recorder, nil, ChatStreamEvent{Type: ChatEventToolStarted, ToolCall: &call}); err != nil {
		t.Fatalf("writeChatEvent: %v", err)
	}
	frame := recorder.Body.String()
	if !strings.HasPrefix(frame, "event: tool_started\ndata: ") {
		t.Fatalf("unexpected frame prefix: %q", frame)
	}
	if !strings.HasSuffix(frame, "\n\n") {
		t.Fatalf("frame must end with a blank line: %q", frame)
	}
	if !strings.Contains(frame, `"name":"search_questions"`) {
		t.Fatalf("frame missing tool call payload: %q", frame)
	}
	if strings.Contains(frame, `"message"`) {
		t.Fatalf("tool event should omit message field: %q", frame)
	}
}

func TestWriteChatEventFramesTokenDelta(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := writeChatEvent(recorder, nil, ChatStreamEvent{Type: ChatEventAssistantDelta, Content: "函数"}); err != nil {
		t.Fatalf("writeChatEvent: %v", err)
	}
	frame := recorder.Body.String()
	if !strings.HasPrefix(frame, "event: assistant_delta\n") {
		t.Fatalf("unexpected frame: %q", frame)
	}
	if !strings.Contains(frame, `"content":"函数"`) {
		t.Fatalf("frame missing delta content: %q", frame)
	}
	if strings.Contains(frame, `"type":"tool_started"`) {
		t.Fatalf("frame should not include tool payload: %q", frame)
	}
}

func TestWriteChatEventIncludesError(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := writeChatEvent(recorder, nil, ChatStreamEvent{Type: ChatEventError, Error: "会话不存在或已被删除"}); err != nil {
		t.Fatalf("writeChatEvent: %v", err)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"会话不存在或已被删除"`) {
		t.Fatalf("frame missing error: %q", recorder.Body.String())
	}
}

func TestStreamAIErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "validation", err: validationError("content", "消息内容不能为空"), want: "消息内容不能为空"},
		{name: "forbidden", err: ErrForbidden, want: "没有操作权限"},
		{name: "not found", err: ErrNotFound, want: "会话不存在或已被删除"},
		{name: "unknown", err: errors.New("internal"), want: "AI 处理失败，请稍后重试"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := streamAIErrorMessage(item.err); got != item.want {
				t.Fatalf("got %q, want %q", got, item.want)
			}
		})
	}
}
