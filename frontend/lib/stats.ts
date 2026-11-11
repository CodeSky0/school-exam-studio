import { z } from "zod";
import { apiRequest } from "./api";

export const questionStatsSchema = z.object({
  draft: z.number().int().nonnegative(),
  published: z.number().int().nonnegative(),
  archived: z.number().int().nonnegative(),
});

export const adminStatsSchema = z.object({
  teachers: z.object({
    active: z.number().int().nonnegative(),
    disabled: z.number().int().nonnegative(),
  }),
  questions: questionStatsSchema,
  papers: z.number().int().nonnegative(),
});

export const teacherStatsSchema = z.object({
  questions: questionStatsSchema,
  papers: z.number().int().nonnegative(),
});

export const statsQueryKeys = {
  admin: ["stats", "admin"] as const,
  teacher: ["stats", "teacher"] as const,
};

export function getAdminStats() {
  return apiRequest("/api/v1/admin/stats", adminStatsSchema);
}

export function getTeacherStats() {
  return apiRequest("/api/v1/teacher/stats", teacherStatsSchema);
}

export type QuestionStats = z.infer<typeof questionStatsSchema>;
export type AdminStats = z.infer<typeof adminStatsSchema>;
export type TeacherStats = z.infer<typeof teacherStatsSchema>;
