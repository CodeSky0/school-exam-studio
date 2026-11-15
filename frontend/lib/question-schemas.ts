import { z } from "zod";
import { richTextHasContent } from "./rich-text";

export const questionTypeSchema = z.enum(["single_choice", "multiple_choice", "true_false", "fill_blank", "essay"]);
export const questionStatusSchema = z.enum(["draft", "published", "archived"]);
export const optionKeySchema = z.enum(["A", "B", "C", "D"]);

const requiredRichTextSchema = z.string().superRefine((value, context) => {
  if (!richTextHasContent(value)) context.addIssue({ code: "custom", message: "请输入内容或公式" });
});

export const questionOptionInputSchema = z.object({
  key: optionKeySchema,
  content: requiredRichTextSchema,
});

export const questionOptionSchema = z.object({
  key: optionKeySchema,
  content: z.string(),
});

export const questionInputSchema = z.object({
  subject: z.string().trim().min(1, "请输入学科").max(30, "学科不能超过 30 个字符"),
  grade: z.string().trim().min(1, "请输入年级").max(30, "年级不能超过 30 个字符"),
  question_type: questionTypeSchema,
  difficulty: z.number().int().min(1, "难度须为 1 至 5").max(5, "难度须为 1 至 5"),
  tags: z.array(z.string().trim().min(1, "标签不能为空").max(30, "每个标签不能超过 30 个字符")).max(10, "标签最多 10 个"),
  category: z.string().trim().max(60, "分类不能超过 60 个字符").default(""),
  knowledge_points: z.array(z.string().trim().min(1).max(30, "每个知识点不能超过 30 个字符")).max(8, "知识点最多 8 个").default([]),
  content: requiredRichTextSchema,
  options: z.array(questionOptionInputSchema),
  answer: z.string(),
  explanation: z.string(),
}).superRefine((value, context) => {
  const isChoice = value.question_type === "single_choice" || value.question_type === "multiple_choice";
  if (isChoice) {
    if (value.options.length < 2 || value.options.length > 4) {
      context.addIssue({ code: "custom", path: ["options"], message: "选择题须保留 2 至 4 个选项" });
    }
    const keys = value.options.map((option) => option.key);
    if (new Set(keys).size !== keys.length) {
      context.addIssue({ code: "custom", path: ["options"], message: "选项键不能重复" });
    }
    if (value.question_type === "single_choice") {
      if (!keys.includes(value.answer as z.infer<typeof optionKeySchema>)) {
        context.addIssue({ code: "custom", path: ["answer"], message: "请选择一个有效答案" });
      }
    } else {
      const answers = value.answer.split(",").map((answer) => answer.trim());
      const hasInvalidAnswer = answers.some((answer) => !keys.includes(answer as z.infer<typeof optionKeySchema>));
      if (!value.answer.trim() || hasInvalidAnswer || new Set(answers).size !== answers.length) {
        context.addIssue({ code: "custom", path: ["answer"], message: "请选择至少一个有效且不重复的答案" });
      }
    }
    return;
  }

  if (value.options.length > 0) {
    context.addIssue({ code: "custom", path: ["options"], message: "当前题型不应包含选项" });
  }
  if (value.question_type === "true_false") {
    if (value.answer !== "true" && value.answer !== "false") {
      context.addIssue({ code: "custom", path: ["answer"], message: "请选择正确或错误" });
    }
  } else if (!richTextHasContent(value.answer)) {
    context.addIssue({ code: "custom", path: ["answer"], message: "请输入参考答案" });
  }
});

export const questionSchema = z.object({
  id: z.string().uuid(),
  creator_id: z.string().uuid(),
  creator_name: z.string(),
  subject: z.string(),
  grade: z.string(),
  question_type: questionTypeSchema,
  difficulty: z.number().int().min(1).max(5),
  tags: z.array(z.string()),
  category: z.string().default(""),
  knowledge_points: z.array(z.string()).default([]),
  content: z.string(),
  options: z.array(questionOptionSchema),
  answer: z.string(),
  explanation: z.string(),
  status: questionStatusSchema,
  ai_generated: z.boolean().default(false),
  ai_meta: z.record(z.string(), z.unknown()).default({}),
  content_hash: z.string().default(""),
  source_question_id: z.string().uuid().nullable().default(null),
  created_at: z.string(),
  updated_at: z.string(),
});

export const questionListSchema = z.object({
  items: z.array(questionSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive().max(100),
});

export const resolvedQuestionsSchema = z.object({ items: z.array(questionSchema) });
export const ocrResponseSchema = z.object({ text: z.string() });

export const questionTypeLabels: Record<QuestionType, string> = {
  single_choice: "单选题",
  multiple_choice: "多选题",
  true_false: "判断题",
  fill_blank: "填空题",
  essay: "简答题",
};

export const questionStatusLabels: Record<QuestionStatus, string> = {
  draft: "草稿",
  published: "已发布",
  archived: "已归档",
};

export function normalizeChoiceAnswer(answer: string): string {
  return Array.from(new Set(answer.toUpperCase().replace(/[^A-D]/g, "").split(""))).sort().join("");
}

export type QuestionType = z.infer<typeof questionTypeSchema>;
export type QuestionStatus = z.infer<typeof questionStatusSchema>;
export type QuestionOption = z.infer<typeof questionOptionSchema>;
export type Question = z.infer<typeof questionSchema>;
export type QuestionInput = z.infer<typeof questionInputSchema>;
export type QuestionList = z.infer<typeof questionListSchema>;
