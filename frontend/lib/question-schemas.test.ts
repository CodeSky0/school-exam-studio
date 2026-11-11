import { describe, expect, it } from "vitest";
import { questionInputSchema, questionSchema, normalizeChoiceAnswer } from "./question-schemas";

const choiceOptions = [
  { key: "A", content: "<p>2</p>" },
  { key: "B", content: "<p>4</p>" },
] as const;

const baseInput = {
  subject: "数学",
  grade: "八年级",
  question_type: "single_choice",
  difficulty: 2,
  tags: ["方程"],
  content: "<p>方程 $x^2=4$ 的正根是</p>",
  options: [...choiceOptions],
  answer: "A",
  explanation: "<p>正数平方根为 2。</p>",
};

describe("题目写入校验", () => {
  it("接受合法单选题并规范化多选答案", () => {
    expect(questionInputSchema.parse(baseInput).answer).toBe("A");
    const multiple = { ...baseInput, question_type: "multiple_choice", answer: "B,A" };
    expect(normalizeChoiceAnswer(multiple.answer)).toBe("AB");
    expect(questionInputSchema.safeParse(multiple).success).toBe(true);
  });

  it("拒绝不存在、重复或未按后端格式分隔的选择题答案", () => {
    expect(questionInputSchema.safeParse({ ...baseInput, answer: "D" }).success).toBe(false);
    expect(questionInputSchema.safeParse({ ...baseInput, question_type: "multiple_choice", answer: "A,A" }).success).toBe(false);
    expect(questionInputSchema.safeParse({ ...baseInput, question_type: "multiple_choice", answer: "AB" }).success).toBe(false);
    expect(questionInputSchema.safeParse({ ...baseInput, question_type: "multiple_choice", answer: "A,C" }).success).toBe(false);
    expect(questionInputSchema.safeParse({ ...baseInput, question_type: "multiple_choice", answer: "A" }).success).toBe(true);
  });

  it("选择题要求二至四个顺序选项且内容非空", () => {
    expect(questionInputSchema.safeParse({ ...baseInput, options: [choiceOptions[0]] }).success).toBe(false);
    expect(questionInputSchema.safeParse({
      ...baseInput,
      options: [{ key: "A", content: "<p><br></p>" }, choiceOptions[1]],
    }).success).toBe(false);
  });

  it("判断题只接受 true 或 false 且不携带选项", () => {
    const judgment = { ...baseInput, question_type: "true_false", options: [], answer: "true" };
    expect(questionInputSchema.safeParse(judgment).success).toBe(true);
    expect(questionInputSchema.safeParse({ ...judgment, answer: "正确" }).success).toBe(false);
    expect(questionInputSchema.safeParse({ ...judgment, options: choiceOptions }).success).toBe(false);
  });

  it("填空和简答需要可编辑答案", () => {
    const fillBlank = { ...baseInput, question_type: "fill_blank", options: [], answer: "<p>$x=2$</p>" };
    expect(questionInputSchema.safeParse(fillBlank).success).toBe(true);
    expect(questionInputSchema.safeParse({ ...fillBlank, answer: "<p></p>" }).success).toBe(false);
  });

  it("校验完整题目响应中的公式属性与分页字段", () => {
    const result = questionSchema.safeParse({
      ...baseInput,
      id: "1f08b234-0a69-4a1f-a2f5-f8a24f2f77a1",
      creator_id: "c2dd2e3f-b0c6-4b0c-a8ce-970e7ac3cf94",
      creator_name: "周老师",
      status: "draft",
      created_at: "2026-11-10T08:00:00Z",
      updated_at: "2026-11-10T08:00:00Z",
    });
    expect(result.success).toBe(true);
  });
});
