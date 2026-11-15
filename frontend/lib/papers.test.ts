import { afterEach, describe, expect, it, vi } from "vitest";
import { createPaper, deletePaper, getPaper, listPapers, serializePaperFilters, updatePaper } from "./papers";
import type { Paper, PaperInput } from "./paper-schemas";

const questionId = "1f08b234-0a69-4a1f-a2f5-f8a24f2f77a1";
const paperId = "47d1f02c-bca9-45b2-a6b1-c9a42971f97e";
const creatorId = "c2dd2e3f-b0c6-4b0c-a8ce-970e7ac3cf94";

const paper: Paper = {
  id: paperId,
  creator_id: creatorId,
  school_name: "明德实验中学",
  title: "八年级数学阶段练习",
  subject: "数学",
  grade: "八年级",
  total_score: "5.00",
  duration_minutes: 90,
  questions: [{
    question_id: questionId,
    order_index: 0,
    assigned_score: "5.00",
    question: {
      id: questionId,
      creator_id: creatorId,
      creator_name: "周老师",
      subject: "数学",
      grade: "八年级",
      question_type: "single_choice",
      difficulty: 2,
      tags: ["方程"],
      category: "方程与不等式",
      knowledge_points: ["一元一次方程"],
      content: "<p>方程的正根是</p>",
      options: [{ key: "A", content: "<p>2</p>" }, { key: "B", content: "<p>4</p>" }],
      answer: "A",
      explanation: "",
      status: "published",
      ai_generated: false,
      ai_meta: {},
      content_hash: "",
      source_question_id: null,
      created_at: "2026-11-10T08:00:00Z",
      updated_at: "2026-11-10T08:00:00Z",
    },
  }],
  created_at: "2026-11-10T08:00:00Z",
  updated_at: "2026-11-10T08:00:00Z",
};

const input: PaperInput = {
  title: paper.title,
  subject: paper.subject,
  grade: paper.grade,
  duration_minutes: 90,
  questions: [{ question_id: questionId, assigned_score: "5.00" }],
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("试卷筛选序列化", () => {
  it("组合学科、年级与分页并清理空白", () => {
    expect(serializePaperFilters({ subject: " 数学 ", grade: " 八年级 ", page: 2, page_size: 20 }).toString())
      .toBe("subject=%E6%95%B0%E5%AD%A6&grade=%E5%85%AB%E5%B9%B4%E7%BA%A7&page=2&page_size=20");
    expect(serializePaperFilters({ subject: " ", page: 1, page_size: 10 }).toString()).toBe("page=1&page_size=10");
  });
});

describe("试卷 API", () => {
  it("读取真实分页列表", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [], total: 0, page: 1, page_size: 10 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(listPapers({ page: 1, page_size: 10, subject: "数学" })).resolves.toEqual({ items: [], total: 0, page: 1, page_size: 10 });
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/papers?subject=%E6%95%B0%E5%AD%A6&page=1&page_size=10",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("读取不含题目明细的列表摘要并使用 question_count", async () => {
    const summary = {
      id: paper.id,
      creator_id: paper.creator_id,
      school_name: paper.school_name,
      title: paper.title,
      subject: paper.subject,
      grade: paper.grade,
      total_score: paper.total_score,
      duration_minutes: paper.duration_minutes,
      question_count: 1,
      created_at: paper.created_at,
      updated_at: paper.updated_at,
    };
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [summary], total: 1, page: 1, page_size: 10 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const result = await listPapers({ page: 1, page_size: 10 });
    expect(result.items).toHaveLength(1);
    expect(result.items[0].question_count).toBe(1);
  });

  it("按顺序创建试卷并发送两位小数字符串分值", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(paper), { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(createPaper(input)).resolves.toEqual(paper);
    expect(fetchMock).toHaveBeenCalledWith("http://localhost:8080/api/v1/papers", expect.objectContaining({
      method: "POST",
      body: JSON.stringify(input),
      credentials: "include",
    }));
  });

  it("读取和更新指定试卷", async () => {
    const fetchMock = vi.fn().mockImplementation(async () => new Response(JSON.stringify(paper), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(getPaper(paperId)).resolves.toEqual(paper);
    await expect(updatePaper(paperId, input)).resolves.toEqual(paper);
    expect(fetchMock).toHaveBeenNthCalledWith(1, `http://localhost:8080/api/v1/papers/${paperId}`, expect.objectContaining({ credentials: "include" }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, `http://localhost:8080/api/v1/papers/${paperId}`, expect.objectContaining({ method: "PATCH", body: JSON.stringify(input) }));
  });

  it("删除指定试卷", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(deletePaper(paperId)).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(`http://localhost:8080/api/v1/papers/${paperId}`, expect.objectContaining({ method: "DELETE" }));
  });
});
