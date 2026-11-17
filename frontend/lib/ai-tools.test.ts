import { afterEach, describe, expect, it, vi } from "vitest";
import { runAISkill, runAITool } from "./ai-tools";

afterEach(() => {
  vi.unstubAllGlobals();
});

function jsonResponse(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("AI 工具客户端", () => {
  it("直接执行工具并补齐默认字段", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ summary: "已体检", data: { total: 3 } }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await runAITool("bank_stats", { limit: 10 });
    expect(result).toMatchObject({ summary: "已体检", data: { total: 3 }, warnings: [] });
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/ai/tools/bank_stats",
      expect.objectContaining({ method: "POST", body: JSON.stringify({ limit: 10 }) }),
    );
  });

  it("一键执行技能", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      skill_id: "paper_quality_check",
      conversation_id: "c-1",
      assistant_message: { id: "m-1", role: "assistant", content: "质检完成" },
      provider: "llm",
      model: "gpt-4o-mini",
    }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await runAISkill("paper_quality_check", { content: "检查这份试卷" });
    expect(result.conversation_id).toBe("c-1");
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/ai/skills/paper_quality_check/run",
      expect.objectContaining({ method: "POST" }),
    );
  });
});
