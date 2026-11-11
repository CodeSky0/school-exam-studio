import { describe, expect, it, vi } from "vitest";
import type { StateStorage } from "zustand/middleware";
import type { Question, QuestionStatus } from "@/lib/question-schemas";
import type { Paper } from "@/lib/paper-schemas";
import { sumScores } from "@/lib/score";
import { createPaperStore, paperCartStorageKey } from "./paper-store";

const userId = "c2dd2e3f-b0c6-4b0c-a8ce-970e7ac3cf94";
const otherUserId = "ee996bd9-d35e-4a62-960c-a853d18d197f";
const firstQuestionId = "1f08b234-0a69-4a1f-a2f5-f8a24f2f77a1";
const secondQuestionId = "ba2dd0d1-97cf-4f52-9597-dc87a85a77a3";

function makeQuestion(id: string, status: QuestionStatus = "published"): Question {
  return {
    id,
    creator_id: userId,
    creator_name: "周老师",
    subject: "数学",
    grade: "八年级",
    question_type: "single_choice",
    difficulty: 2,
    tags: ["方程"],
    content: `<p>题目 ${id}</p>`,
    options: [{ key: "A", content: "<p>2</p>" }, { key: "B", content: "<p>4</p>" }],
    answer: "A",
    explanation: "",
    status,
    created_at: "2026-11-10T08:00:00Z",
    updated_at: "2026-11-10T08:00:00Z",
  };
}

function memoryStorage(): StateStorage {
  const values = new Map<string, string>();
  return {
    getItem: (name) => values.get(name) ?? null,
    setItem: (name, value) => { values.set(name, value); },
    removeItem: (name) => { values.delete(name); },
  };
}

function options(storage = memoryStorage(), resolveQuestions = async () => [] as Question[]) {
  return { storage, resolveQuestions };
}

describe("组卷购物车", () => {
  it("只添加已发布题目并按 ID 去重", () => {
    const store = createPaperStore(userId, options());
    const question = makeQuestion(firstQuestionId);
    expect(store.getState().addQuestion(question)).toBe(true);
    expect(store.getState().addQuestion(question)).toBe(false);
    expect(store.getState().addQuestion(makeQuestion(secondQuestionId, "draft"))).toBe(false);
    expect(store.getState().items).toHaveLength(1);
  });

  it("使用两位小数字符串并用整数百分实时汇总", () => {
    const store = createPaperStore(userId, options());
    store.getState().addQuestion(makeQuestion(firstQuestionId));
    store.getState().addQuestion(makeQuestion(secondQuestionId));
    store.getState().setScore(firstQuestionId, "0.10");
    store.getState().setScore(secondQuestionId, "0.20");
    expect(sumScores(store.getState().items.map((item) => item.assignedScore))).toBe("0.30");
    store.getState().setScore(firstQuestionId, "1.234");
    expect(store.getState().items[0].assignedScore).toBe("0.10");
  });

  it("支持拖拽重排及边界安全的上移下移", () => {
    const store = createPaperStore(userId, options());
    store.getState().addQuestion(makeQuestion(firstQuestionId));
    store.getState().addQuestion(makeQuestion(secondQuestionId));
    store.getState().moveQuestion(secondQuestionId, -1);
    expect(store.getState().items.map((item) => item.question.id)).toEqual([secondQuestionId, firstQuestionId]);
    store.getState().moveQuestion(secondQuestionId, -1);
    expect(store.getState().items[0].question.id).toBe(secondQuestionId);
    store.getState().reorderQuestion(secondQuestionId, firstQuestionId);
    expect(store.getState().items.map((item) => item.question.id)).toEqual([firstQuestionId, secondQuestionId]);
    store.getState().removeQuestion(firstQuestionId);
    expect(store.getState().items.map((item) => item.question.id)).toEqual([secondQuestionId]);
  });

  it("持久化键包含用户 ID 且不同用户互不共享", () => {
    const storage = memoryStorage();
    const first = createPaperStore(userId, options(storage));
    const second = createPaperStore(otherUserId, options(storage));
    first.getState().addQuestion(makeQuestion(firstQuestionId));
    expect(paperCartStorageKey(userId)).toContain(userId);
    expect(paperCartStorageKey(userId)).not.toBe(paperCartStorageKey(otherUserId));
    expect(storage.getItem(paperCartStorageKey(userId))).toContain(firstQuestionId);
    expect(second.getState().items).toEqual([]);
  });

  it("恢复时调用 resolve，保留顺序并移除失效题目", async () => {
    const storage = memoryStorage();
    const first = makeQuestion(firstQuestionId);
    const second = makeQuestion(secondQuestionId);
    storage.setItem(paperCartStorageKey(userId), JSON.stringify({
      state: {
        paperId: null,
        title: "待恢复试卷",
        subject: "数学",
        grade: "八年级",
        durationMinutes: "90",
        items: [
          { question: first, assignedScore: "0.10" },
          { question: second, assignedScore: "0.20" },
        ],
      },
      version: 1,
    }));
    const resolveQuestions = vi.fn(async () => [first]);
    const store = createPaperStore(userId, options(storage, resolveQuestions));
    await store.persist.rehydrate();
    await vi.waitFor(() => {
      expect(store.getState().items.map((item) => item.question.id)).toEqual([firstQuestionId]);
      expect(store.getState().isResolving).toBe(false);
    });
    expect(resolveQuestions).toHaveBeenCalledWith([firstQuestionId, secondQuestionId]);
    expect(store.getState().removedInvalidCount).toBe(1);
    expect(store.getState().items[0].assignedScore).toBe("0.10");
  });

  it("载入已有试卷时按 order_index 排序", () => {
    const store = createPaperStore(userId, options());
    const first = makeQuestion(firstQuestionId);
    const second = makeQuestion(secondQuestionId);
    const paper: Paper = {
      id: "47d1f02c-bca9-45b2-a6b1-c9a42971f97e",
      creator_id: userId,
      school_name: "明德实验中学",
      title: "阶段练习",
      subject: "数学",
      grade: "八年级",
      total_score: "0.30",
      duration_minutes: 90,
      questions: [
        { question_id: secondQuestionId, order_index: 1, assigned_score: "0.20", question: second },
        { question_id: firstQuestionId, order_index: 0, assigned_score: "0.10", question: first },
      ],
      created_at: "2026-11-10T08:00:00Z",
      updated_at: "2026-11-10T08:00:00Z",
    };
    store.getState().loadPaper(paper);
    expect(store.getState().paperId).toBe(paper.id);
    expect(store.getState().items.map((item) => item.question.id)).toEqual([firstQuestionId, secondQuestionId]);
    expect(store.getState().durationMinutes).toBe("90");
  });
});
