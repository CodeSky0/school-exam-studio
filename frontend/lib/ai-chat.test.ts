import { describe, expect, it } from "vitest";
import {
  attachmentKindLabel,
  chatMessageSchema,
  formatBytes,
  formatToolDuration,
  messageQuestionIDs,
  toolCallSchema,
  toolStatusLabel,
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