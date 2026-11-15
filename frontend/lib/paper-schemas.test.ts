import { describe, expect, it } from "vitest";
import { paperInputSchema, paperListSchema, paperSchema } from "./paper-schemas";

const questionId = "1f08b234-0a69-4a1f-a2f5-f8a24f2f77a1";
const creatorId = "c2dd2e3f-b0c6-4b0c-a8ce-970e7ac3cf94";

const question = {
  id: questionId,
  creator_id: creatorId,
  creator_name: "周老师",
  subject: "数学",
  grade: "八年级",
  question_type: "single_choice",
  difficulty: 2,
  tags: ["方程"],
  category: "",
  knowledge_points: [],
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
};

const paper = {
  id: "47d1f02c-bca9-45b2-a6b1-c9a42971f97e",
  creator_id: creatorId,
  school_name: "明德实验中学",
  title: "八年级数学阶段练习",
  subject: "数学",
  grade: "八年级",
  total_score: "5.00",
  duration_minutes: 90,
  questions: [{ question_id: questionId, order_index: 0, assigned_score: "5.00", question }],
  created_at: "2026-11-10T08:00:00Z",
  updated_at: "2026-11-10T08:00:00Z",
};

const input = {
  title: paper.title,
  subject: paper.subject,
  grade: paper.grade,
  duration_minutes: 90,
  questions: [{ question_id: questionId, assigned_score: "5.00" }],
};

describe("试卷 Zod 契约", () => {
  it("接受完整试卷与合法写入数据", () => {
    expect(paperSchema.parse(paper)).toEqual(paper);
    expect(paperInputSchema.parse(input)).toEqual(input);
  });

  it("要求至少一题且题目不能重复", () => {
    expect(paperInputSchema.safeParse({ ...input, questions: [] }).success).toBe(false);
    expect(paperInputSchema.safeParse({ ...input, questions: [...input.questions, ...input.questions] }).success).toBe(false);
  });

  it.each(["0.00", "5", "5.0", "5.000", "-5.00", "05.00"])("拒绝非法试卷分值 %s", (assignedScore) => {
    expect(paperInputSchema.safeParse({
      ...input,
      questions: [{ question_id: questionId, assigned_score: assignedScore }],
    }).success).toBe(false);
  });

  it("拒绝超过 NUMERIC(8,2) 上限的试卷总分", () => {
    expect(paperInputSchema.safeParse({
      ...input,
      questions: [
        { question_id: questionId, assigned_score: "999999.99" },
        { question_id: "ba2dd0d1-97cf-4f52-9597-dc87a85a77a3", assigned_score: "0.01" },
      ],
    }).success).toBe(false);
  });

  it("限制考试时长为 1 至 600 分钟", () => {
    expect(paperInputSchema.safeParse({ ...input, duration_minutes: 0 }).success).toBe(false);
    expect(paperInputSchema.safeParse({ ...input, duration_minutes: 601 }).success).toBe(false);
    expect(paperInputSchema.safeParse({ ...input, duration_minutes: 600 }).success).toBe(true);
  });

  it("拒绝结构不完整的试卷响应", () => {
    expect(paperSchema.safeParse({ ...paper, school_name: undefined }).success).toBe(false);
    expect(paperSchema.safeParse({ ...paper, total_score: 5 }).success).toBe(false);
  });

  it("列表摘要不要求题目明细并使用 question_count 展示题数", () => {
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
    const parsed = paperListSchema.parse({ items: [summary], total: 1, page: 1, page_size: 10 });
    expect(parsed.items[0].question_count).toBe(1);

    const legacy = paperListSchema.parse({ items: [{ ...summary, question_count: undefined }], total: 1, page: 1, page_size: 10 });
    expect(legacy.items[0].question_count).toBe(0);
  });
});
