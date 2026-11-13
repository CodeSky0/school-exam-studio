import { z } from "zod";
import { questionSchema } from "./question-schemas";
import { sumScores, toCents } from "./score";

export const paperScoreSchema = z.string().superRefine((value, context) => {
  try {
    toCents(value);
  } catch (error) {
    context.addIssue({
      code: "custom",
      message: error instanceof Error ? error.message : "请输入有效分值",
    });
  }
});

export const paperQuestionInputSchema = z.object({
  question_id: z.string().uuid(),
  assigned_score: paperScoreSchema,
});

export const paperInputSchema = z.object({
  title: z.string().trim().min(1, "请输入试卷标题").max(200, "试卷标题不能超过 200 个字符"),
  subject: z.string().trim().min(1, "请输入学科").max(30, "学科不能超过 30 个字符"),
  grade: z.string().trim().min(1, "请输入年级").max(30, "年级不能超过 30 个字符"),
  duration_minutes: z.number().int("考试时长须为整数").min(1, "考试时长至少为 1 分钟").max(600, "考试时长不能超过 600 分钟"),
  questions: z.array(paperQuestionInputSchema).min(1, "请至少添加一道题目"),
}).superRefine((value, context) => {
  const ids = value.questions.map((item) => item.question_id);
  if (new Set(ids).size !== ids.length) {
    context.addIssue({ code: "custom", path: ["questions"], message: "试卷中不能重复添加同一道题" });
  }
  if (value.questions.every((item) => paperScoreSchema.safeParse(item.assigned_score).success)) {
    try {
      sumScores(value.questions.map((item) => item.assigned_score));
    } catch {
      context.addIssue({ code: "custom", path: ["questions"], message: "试卷总分不能超过 999999.99 分" });
    }
  }
});

export const paperQuestionSchema = z.object({
  question_id: z.string().uuid(),
  order_index: z.number().int().nonnegative(),
  assigned_score: paperScoreSchema,
  question: questionSchema,
});

export const paperSchema = z.object({
  id: z.string().uuid(),
  creator_id: z.string().uuid(),
  school_name: z.string().min(1),
  title: z.string().min(1).max(200),
  subject: z.string().min(1).max(30),
  grade: z.string().min(1).max(30),
  total_score: paperScoreSchema,
  duration_minutes: z.number().int().min(1).max(600),
  questions: z.array(paperQuestionSchema),
  created_at: z.string(),
  updated_at: z.string(),
});

export const paperSummarySchema = paperSchema.omit({ questions: true }).extend({
  question_count: z.number().int().nonnegative().default(0),
});

export const paperListSchema = z.object({
  items: z.array(paperSummarySchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive().max(100),
});

export type PaperQuestionInput = z.infer<typeof paperQuestionInputSchema>;
export type PaperInput = z.infer<typeof paperInputSchema>;
export type PaperQuestion = z.infer<typeof paperQuestionSchema>;
export type Paper = z.infer<typeof paperSchema>;
export type PaperSummary = z.infer<typeof paperSummarySchema>;
export type PaperList = z.infer<typeof paperListSchema>;
