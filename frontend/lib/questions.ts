import { z } from "zod";
import { apiRequest } from "./api";
import {
  questionListSchema,
  questionSchema,
  resolvedQuestionsSchema,
  type QuestionInput,
  type QuestionStatus,
  type QuestionType,
} from "./question-schemas";

export interface QuestionFilters {
  subject?: string;
  grade?: string;
  question_type?: QuestionType;
  difficulty?: number;
  category?: string;
  knowledge_point?: string;
  tag?: string;
  creator_id?: string;
  status?: QuestionStatus;
  page: number;
  page_size: number;
}

export const questionQueryKeys = {
  all: ["questions"] as const,
  lists: () => [...questionQueryKeys.all, "list"] as const,
  list: (filters: QuestionFilters) => [...questionQueryKeys.lists(), filters] as const,
  detail: (id: string) => [...questionQueryKeys.all, "detail", id] as const,
};

export function serializeQuestionFilters(filters: QuestionFilters): URLSearchParams {
  const query = new URLSearchParams();
  const appendText = (key: string, value: string | undefined) => {
    const trimmed = value?.trim();
    if (trimmed) query.set(key, trimmed);
  };
  appendText("subject", filters.subject);
  appendText("grade", filters.grade);
  if (filters.question_type) query.set("question_type", filters.question_type);
  if (filters.difficulty) query.set("difficulty", String(filters.difficulty));
  appendText("category", filters.category);
  appendText("knowledge_point", filters.knowledge_point);
  appendText("tag", filters.tag);
  appendText("creator_id", filters.creator_id);
  if (filters.status) query.set("status", filters.status);
  query.set("page", String(filters.page));
  query.set("page_size", String(filters.page_size));
  return query;
}

export const batchFailureSchema = z.object({
  id: z.string(),
  error: z.string(),
});

export const batchStatusResultSchema = z.object({
  status: z.string(),
  total: z.number().int().default(0),
  succeeded: z.array(z.string()).default([]),
  failed: z.array(batchFailureSchema).default([]),
});

export const batchDeleteResultSchema = z.object({
  total: z.number().int().default(0),
  deleted: z.array(z.string()).default([]),
  failed: z.array(batchFailureSchema).default([]),
});

export type BatchStatusResult = z.infer<typeof batchStatusResultSchema>;
export type BatchDeleteResult = z.infer<typeof batchDeleteResultSchema>;

export function batchUpdateQuestionStatus(ids: string[], status: "published" | "archived") {
  return apiRequest("/api/v1/admin/questions/batch/status", batchStatusResultSchema, {
    method: "PATCH",
    body: JSON.stringify({ ids, status }),
  });
}

export function batchDeleteQuestions(ids: string[]) {
  return apiRequest("/api/v1/questions/batch/delete", batchDeleteResultSchema, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

export function listQuestions(filters: QuestionFilters) {
  const query = serializeQuestionFilters(filters);
  return apiRequest(`/api/v1/questions?${query.toString()}`, questionListSchema);
}

export function getQuestion(id: string) {
  return apiRequest(`/api/v1/questions/${encodeURIComponent(id)}`, questionSchema);
}

export function createQuestion(input: QuestionInput) {
  return apiRequest("/api/v1/questions", questionSchema, { method: "POST", body: JSON.stringify(input) });
}

export function updateQuestion(id: string, input: QuestionInput) {
  return apiRequest(`/api/v1/questions/${encodeURIComponent(id)}`, questionSchema, { method: "PATCH", body: JSON.stringify(input) });
}

export function updateQuestionStatus(id: string, status: "published" | "archived") {
  return apiRequest(`/api/v1/admin/questions/${encodeURIComponent(id)}/status`, questionSchema, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

export async function deleteQuestion(id: string): Promise<void> {
  await apiRequest(`/api/v1/questions/${encodeURIComponent(id)}`, z.unknown(), { method: "DELETE" });
}

export async function resolveQuestions(ids: string[]) {
  const response = await apiRequest("/api/v1/questions/resolve", resolvedQuestionsSchema, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
  return response.items;
}
