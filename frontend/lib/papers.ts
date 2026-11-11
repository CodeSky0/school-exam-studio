import { z } from "zod";
import { apiRequest } from "./api";
import { paperListSchema, paperSchema, type PaperInput } from "./paper-schemas";

export interface PaperFilters {
  subject?: string;
  grade?: string;
  page: number;
  page_size: number;
}

export const paperQueryKeys = {
  all: ["papers"] as const,
  lists: () => [...paperQueryKeys.all, "list"] as const,
  list: (filters: PaperFilters) => [...paperQueryKeys.lists(), filters] as const,
  detail: (id: string) => [...paperQueryKeys.all, "detail", id] as const,
};

export function serializePaperFilters(filters: PaperFilters): URLSearchParams {
  const query = new URLSearchParams();
  const subject = filters.subject?.trim();
  const grade = filters.grade?.trim();
  if (subject) query.set("subject", subject);
  if (grade) query.set("grade", grade);
  query.set("page", String(filters.page));
  query.set("page_size", String(filters.page_size));
  return query;
}

export function listPapers(filters: PaperFilters) {
  const query = serializePaperFilters(filters);
  return apiRequest(`/api/v1/papers?${query.toString()}`, paperListSchema);
}

export function getPaper(id: string) {
  return apiRequest(`/api/v1/papers/${encodeURIComponent(id)}`, paperSchema);
}

export function createPaper(input: PaperInput) {
  return apiRequest("/api/v1/papers", paperSchema, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updatePaper(id: string, input: PaperInput) {
  return apiRequest(`/api/v1/papers/${encodeURIComponent(id)}`, paperSchema, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export async function deletePaper(id: string): Promise<void> {
  await apiRequest(`/api/v1/papers/${encodeURIComponent(id)}`, z.unknown(), { method: "DELETE" });
}
