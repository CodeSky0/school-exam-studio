import { afterEach, describe, expect, it, vi } from "vitest";
import {
  attachmentKindLabel,
  chatMessageSchema,
  chatStreamEventSchema,
  formatBytes,
  formatToolDuration,
  messageQuestionIDs,
  processSummary,
  streamMessage,
  toolCallSchema,
  toolStatusLabel,
  type ChatStreamEvent,
} from "./ai-chat";

describe("ai chat schemas", () => {
  it("parses a message with tool calls and attachments", () => {
    const message = chatMessageSchema.parse({
      id: "m1",
      conversation_id: "c1",
      role: "assistant",
      content: "已完成",
      status: "succeeded",
      provider: "llm",
      model: "gpt-4o-mini",
      tool_calls: [{
        id: "t1",
        name: "bank_audit",
        label: "题库体检",
        status: "succeeded",
        summary: "检查 10 道题",
        arguments: {},
        result: { question_ids: ["q1"] },
      }],
      attachments: [{
        id: "a1",
        name: "试卷.pdf",
        mime: "application/pdf",
        kind: "pdf",
        status: "parsed",
        text_chars: 120,
      }],
      created_at: "2026-11-17T10:00:00Z",
    });
    expect(message.tool_calls).toHaveLength(1);
    expect(message.attachments[0].name).toBe("试卷.pdf");
    expect(message.created_at).toContain("2026");
  });

  it("rejects unknown tool status", () => {
    const parsed = toolCallSchema.safeParse({
      id: "t1", name: "x", label: "x", status: "exploded", arguments: {},
    });
    expect(parsed.success).toBe(false);
  });
});

function sseResponse(frames: string[]) {
  const encoder = new TextEncoder();
  return new Response(new ReadableStream({
    start(controller) {
      for (const frame of frames) controller.enqueue(encoder.encode(frame));
      controller.close();
    },
  }), { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("streamMessage", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("dispatches frames even when chunks split mid-event", async () => {
    const fetchMock = vi.fn().mockResolvedValue(sseResponse([
      'event: user_message\ndata: {"type":"user_message","message":{"id":"m1","conversation_id":"c1","role":"user","created_at":"2026-11-17T10:00:00Z"}}\n\n',
      'event: tool_started\ndata: {"type":"tool_started","tool_call":{"id":"t1","name":"search_questions","label":"检索题目","status":"running","arguments":{}}}\n\nevent: tool_f',
      'inished\ndata: {"type":"tool_finished","tool_call":{"id":"t1","name":"search_questions","label":"检索题目","status":"succeeded","summary":"找到 2 道","arguments":{}}}\n\n',
      'event: done\ndata: {"type":"done"}\n\n',
    ]));
    vi.stubGlobal("fetch", fetchMock);
    const events: ChatStreamEvent[] = [];
    await streamMessage("c1", { content: "找题" }, (event) => events.push(event));
    expect(events.map((event) => event.type)).toEqual(["user_message", "tool_started", "tool_finished", "done"]);
    expect(events[2].tool_call?.summary).toBe("找到 2 道");
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/ai/conversations/c1/messages/stream",
      expect.objectContaining({ method: "POST", body: JSON.stringify({ content: "找题", attachment_ids: [], model: "" }) }),
    );
  });

  it("parses token deltas before the final assistant message", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(sseResponse([
      'event: reasoning_delta\ndata: {"type":"reasoning_delta","reasoning":"先检索"}\n\n',
      'event: assistant_delta\ndata: {"type":"assistant_delta","content":"找到"}\n\n',
      'event: assistant_delta\ndata: {"type":"assistant_delta","content":" 8 道题"}\n\n',
      'event: assistant_message\ndata: {"type":"assistant_message","message":{"id":"m2","conversation_id":"c1","role":"assistant","content":"找到 8 道题","created_at":"2026-11-17T10:00:00Z"}}\n\n',
      'event: done\ndata: {"type":"done"}\n\n',
    ])));
    const events: ChatStreamEvent[] = [];
    await streamMessage("c1", { content: "找题" }, (event) => events.push(event));
    expect(events.map((event) => event.type)).toEqual([
      "reasoning_delta", "assistant_delta", "assistant_delta", "assistant_message", "done",
    ]);
    expect(events.filter((event) => event.content).map((event) => event.content).join("")).toBe("找到 8 道题");
    expect(events[0].reasoning).toBe("先检索");
  });

  it("parses a token delta event schema", () => {
    const event = chatStreamEventSchema.parse({ type: "assistant_delta", content: "函数" });
    expect(event.content).toBe("函数");
  });

  it("throws the API error message for non-2xx responses", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(
      JSON.stringify({ error: { code: "BAD_REQUEST", message: "消息内容不能为空" }, request_id: "req-1" }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    )));
    await expect(streamMessage("c1", { content: "" }, () => {})).rejects.toThrow("消息内容不能为空");
  });
});

describe("ai chat helpers", () => {
  it("formats file sizes", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatBytes(3 * 1024 * 1024)).toBe("3.0 MB");
  });

  it("formats tool durations", () => {
    expect(formatToolDuration("2026-11-17T10:00:00Z", "2026-11-17T10:00:00.250Z")).toBe("250 ms");
    expect(formatToolDuration("2026-11-17T10:00:00Z", "2026-11-17T10:00:02Z")).toBe("2.0 s");
    expect(formatToolDuration(undefined, undefined)).toBe("");
  });

  it("summarizes process duration and tool count", () => {
    const message = chatMessageSchema.parse({
      id: "m1", conversation_id: "c1", role: "assistant", content: "完成",
      created_at: "2026-11-17T10:00:08Z",
      timeline: [{ type: "thought", content: "先检索题库", created_at: "2026-11-17T10:00:00Z" }],
      tool_calls: [{
        id: "t1", name: "search_questions", label: "检索题目", status: "succeeded", summary: "",
        arguments: {}, started_at: "2026-11-17T10:00:01Z", finished_at: "2026-11-17T10:00:05Z",
      }],
    });
    expect(processSummary(message, false)).toBe("运行了 8 秒，调用 1 次工具");
    expect(processSummary(message, true)).toBe("正在运行 · 已调用 1 次工具");
  });

  it("summarizes thought-only processes", () => {
    const message = chatMessageSchema.parse({
      id: "m1", conversation_id: "c1", role: "assistant", content: "完成",
      created_at: "2026-11-17T10:00:03Z",
      timeline: [{ type: "thought", content: "先分析", created_at: "2026-11-17T10:00:00Z" }],
    });
    expect(processSummary(message, false)).toBe("运行了 3 秒，完成思考");
    expect(processSummary(message, true)).toBe("正在思考…");
  });

  it("labels tool status and attachment kind", () => {
    expect(toolStatusLabel("failed")).toBe("失败");
    expect(attachmentKindLabel({ kind: "docx" } as never)).toBe("Word");
  });

  it("collects question ids from tool results", () => {
    const message = chatMessageSchema.parse({
      id: "m1", conversation_id: "c1", role: "assistant", content: "", created_at: "2026-11-17T10:00:00Z",
      tool_calls: [{
        id: "t1", name: "import_document_questions", label: "文档入库", status: "succeeded", summary: "",
        arguments: {}, result: { question_ids: ["q1", "q2"], data: { question_ids: ["q2", "q3"] } },
      }],
    });
    expect(messageQuestionIDs(message).sort()).toEqual(["q1", "q2", "q3"]);
  });
});