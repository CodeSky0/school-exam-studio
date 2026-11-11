import { afterEach, describe, expect, it, vi } from "vitest";
import { listQuestions, serializeQuestionFilters } from "./questions";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("题目筛选序列化", () => {
  it("组合所有非空筛选并保留分页", () => {
    const query = serializeQuestionFilters({
      subject: " 数学 ",
      grade: "八年级",
      question_type: "single_choice",
      difficulty: 3,
      tag: " 方程 ",
      creator_id: "c2dd2e3f-b0c6-4b0c-a8ce-970e7ac3cf94",
      status: "draft",
      page: 2,
      page_size: 20,
    });
    expect(query.toString()).toBe("subject=%E6%95%B0%E5%AD%A6&grade=%E5%85%AB%E5%B9%B4%E7%BA%A7&question_type=single_choice&difficulty=3&tag=%E6%96%B9%E7%A8%8B&creator_id=c2dd2e3f-b0c6-4b0c-a8ce-970e7ac3cf94&status=draft&page=2&page_size=20");
  });

  it("忽略空白筛选", () => {
    const query = serializeQuestionFilters({ subject: "  ", page: 1, page_size: 10 });
    expect(query.toString()).toBe("page=1&page_size=10");
  });
});

describe("题目 API", () => {
  it("请求真实列表并校验响应", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [], total: 0, page: 1, page_size: 10 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(listQuestions({ page: 1, page_size: 10, status: "draft" })).resolves.toEqual({ items: [], total: 0, page: 1, page_size: 10 });
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/questions?status=draft&page=1&page_size=10",
      expect.objectContaining({ credentials: "include" }),
    );
  });
});
