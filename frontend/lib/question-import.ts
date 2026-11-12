import { z } from "zod";
import { apiDownload, apiRequest } from "./api";
import { questionOptionSchema, questionTypeSchema } from "./question-schemas";

export const CSV_MAX_FILE_SIZE = 5 * 1024 * 1024;

const nullableStringArraySchema = z.array(z.string()).nullable().transform((value) => value ?? []);
const nullableOptionsSchema = z.array(questionOptionSchema).nullable().transform((value) => value ?? []);

export const importPreviewRowSchema = z.object({
  line: z.number().int().positive().optional(),
  subject: z.string(),
  grade: z.string(),
  question_type: questionTypeSchema.optional(),
  type: questionTypeSchema.optional(),
  difficulty: z.number().int().min(1).max(5),
  tags: nullableStringArraySchema,
  content: z.string(),
  options: nullableOptionsSchema,
  answer: z.string(),
  explanation: z.string(),
}).refine((row) => Boolean(row.question_type ?? row.type), { message: "缺少题型" }).transform((row) => ({
  ...row,
  question_type: (row.question_type ?? row.type)!,
}));

export const importRowErrorSchema = z.object({
  line: z.number().int().positive(),
  field: z.string(),
  message: z.string().min(1),
});

export const importValidationResultSchema = z.object({
  valid_count: z.number().int().nonnegative(),
  preview: z.array(importPreviewRowSchema).max(50),
  errors: z.array(importRowErrorSchema),
});

export const importResultSchema = z.object({ imported: z.number().int().nonnegative() });

export function validateCsvFile(file: File): string | null {
  const acceptedTypes = new Set(["", "text/csv", "application/csv", "application/vnd.ms-excel", "text/plain"]);
  if (!file.name.toLowerCase().endsWith(".csv") || !acceptedTypes.has(file.type)) return "请选择 CSV 格式文件";
  if (file.size === 0) return "CSV 文件内容为空，请重新选择";
  if (file.size > CSV_MAX_FILE_SIZE) return "CSV 文件不能超过 5 MB";
  return null;
}

function csvFormData(file: File): FormData {
  const body = new FormData();
  body.append("file", file);
  return body;
}

export function validateQuestionImport(file: File) {
  return apiRequest("/api/v1/questions/import/validate", importValidationResultSchema, {
    method: "POST",
    body: csvFormData(file),
  }, importValidationResultSchema);
}

export function importQuestions(file: File) {
  return apiRequest("/api/v1/questions/import", importResultSchema, {
    method: "POST",
    body: csvFormData(file),
  }, importValidationResultSchema);
}

export function downloadQuestionImportTemplate() {
  return apiDownload("/api/v1/questions/import/template");
}

export type ImportPreviewRow = z.infer<typeof importPreviewRowSchema>;
export type ImportValidationResult = z.infer<typeof importValidationResultSchema>;
