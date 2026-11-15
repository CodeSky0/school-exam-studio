import { z } from "zod";
import { apiRequest } from "./api";
import { currentUserSchema, type CurrentUser } from "./schemas";

export const teacherListSchema = z.object({
  items: z.array(currentUserSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive().max(100),
});

export const schoolSchema = z.object({
  id: z.string().uuid(),
  name: z.string().min(1),
  school_code: z.string().regex(/^\d{8}$/),
  created_at: z.string(),
  updated_at: z.string(),
});

export const resetPasswordResponseSchema = z.object({ temporary_password: z.string().min(1) });

export interface TeacherFilters {
  subject?: string;
  status?: "active" | "disabled";
  page: number;
  page_size: number;
}

export const schoolAISettingsSchema = z.object({
  provider: z.string(),
  base_url: z.string(),
  api_key_hint: z.string().default(""),
  has_api_key: z.boolean(),
  models: z.array(z.string()).default([]),
  default_model: z.string().default(""),
  enabled: z.boolean(),
  configured: z.boolean(),
  source: z.string(),
  warning: z.string().optional(),
  updated_at: z.string().nullable().optional(),
});

export interface SchoolAISettingsInput {
  provider: string;
  base_url: string;
  api_key?: string;
  models: string[];
  default_model: string;
  enabled: boolean;
  clear_api_key?: boolean;
}

export const adminQueryKeys = {
  teachers: ["admin", "teachers"] as const,
  teacherList: (filters: TeacherFilters) => [...adminQueryKeys.teachers, filters] as const,
  teacherOptions: ["admin", "teacher-options"] as const,
  school: ["admin", "school"] as const,
  aiSettings: ["admin", "ai-settings"] as const,
};

export function serializeTeacherFilters(filters: TeacherFilters): URLSearchParams {
  const query = new URLSearchParams();
  const subject = filters.subject?.trim();
  if (subject) query.set("subject", subject);
  if (filters.status) query.set("status", filters.status);
  query.set("page", String(filters.page));
  query.set("page_size", String(filters.page_size));
  return query;
}

export function listTeachers(filters: TeacherFilters) {
  return apiRequest(`/api/v1/admin/teachers?${serializeTeacherFilters(filters).toString()}`, teacherListSchema);
}

const ALL_TEACHERS_PAGE_SIZE = 100;

export async function listAllTeachers(): Promise<CurrentUser[]> {
  const firstPage = await listTeachers({ page: 1, page_size: ALL_TEACHERS_PAGE_SIZE });
  const pageCount = Math.ceil(firstPage.total / ALL_TEACHERS_PAGE_SIZE);
  const remainingPages = pageCount > 1
    ? await Promise.all(Array.from({ length: pageCount - 1 }, (_, index) => listTeachers({ page: index + 2, page_size: ALL_TEACHERS_PAGE_SIZE })))
    : [];
  const teachers = new Map<string, CurrentUser>();
  for (const teacher of [firstPage, ...remainingPages].flatMap((page) => page.items)) {
    if (!teachers.has(teacher.id)) teachers.set(teacher.id, teacher);
  }
  return Array.from(teachers.values());
}

export function updateTeacherStatus(id: string, status: "active" | "disabled") {
  return apiRequest(`/api/v1/admin/teachers/${encodeURIComponent(id)}/status`, currentUserSchema, {
    method: "PATCH",
    body: JSON.stringify({ status }),
  });
}

export function resetTeacherPassword(id: string) {
  return apiRequest(`/api/v1/admin/teachers/${encodeURIComponent(id)}/reset-password`, resetPasswordResponseSchema, { method: "POST" });
}

export function getSchool() {
  return apiRequest("/api/v1/admin/school", schoolSchema);
}

export function updateSchoolName(name: string) {
  return apiRequest("/api/v1/admin/school", schoolSchema, { method: "PATCH", body: JSON.stringify({ name }) });
}

export function getSchoolAISettings() {
  return apiRequest("/api/v1/admin/ai/settings", schoolAISettingsSchema);
}

export function updateSchoolAISettings(input: SchoolAISettingsInput) {
  return apiRequest("/api/v1/admin/ai/settings", schoolAISettingsSchema, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export type TeacherList = z.infer<typeof teacherListSchema>;
export type School = z.infer<typeof schoolSchema>;
export type SchoolAISettings = z.infer<typeof schoolAISettingsSchema>;
