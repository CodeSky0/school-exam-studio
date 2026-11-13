import path from "node:path";
import fs from "node:fs";
import { fileURLToPath } from "node:url";
import { request, type APIRequestContext } from "@playwright/test";

const dirname = path.dirname(fileURLToPath(import.meta.url));

export const API_URL = (process.env.E2E_API_URL ?? "http://localhost:8080").replace(/\/$/, "");
export const APP_ORIGIN = process.env.E2E_APP_ORIGIN ?? "http://localhost:3000";
export const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? "admin-e2e@example.com";
export const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? "AdminE2E!2026Secure";
export const TEACHER_EMAIL = process.env.E2E_TEACHER_EMAIL ?? "teacher-e2e@example.com";
export const TEACHER_PASSWORD = process.env.E2E_TEACHER_PASSWORD ?? "TeacherE2E!2026Secure";

export const authDirectory = path.join(dirname, ".auth");
export const adminStorage = path.join(authDirectory, "admin.json");
export const teacherStorage = path.join(authDirectory, "teacher.json");
const metaFile = path.join(authDirectory, "meta.json");

export interface E2EMeta {
  schoolCode: string | null;
  adminId: string;
  teacherId: string;
}

export function readMeta(): E2EMeta {
  return JSON.parse(fs.readFileSync(metaFile, "utf8")) as E2EMeta;
}

export function storedMeta(): E2EMeta | null {
  return fs.existsSync(metaFile) ? readMeta() : null;
}

export function writeMeta(meta: E2EMeta): void {
  fs.writeFileSync(metaFile, JSON.stringify(meta, null, 2));
}

export async function apiContext(storageState?: string): Promise<APIRequestContext> {
  return request.newContext({
    baseURL: API_URL,
    extraHTTPHeaders: { Origin: APP_ORIGIN },
    storageState,
  });
}

export function uniqueLabel(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}${Math.floor(Math.random() * 1e6).toString(36)}`;
}

export interface CreatedQuestion {
  id: string;
  status: string;
}

export async function createQuestion(
  api: APIRequestContext,
  overrides: Record<string, unknown> = {},
): Promise<CreatedQuestion> {
  const marker = uniqueLabel("e2e");
  const response = await api.post("/api/v1/questions", {
    data: {
      subject: "数学",
      grade: "七年级",
      question_type: "single_choice",
      difficulty: 1,
      tags: [marker],
      content: `<p>${marker} 求方程 $x^2=4$ 的正根</p>`,
      options: [
        { key: "A", content: "<p>2</p>" },
        { key: "B", content: "<p>-2</p>" },
      ],
      answer: "A",
      explanation: "<p>平方根定义</p>",
      ...overrides,
    },
  });
  if (!response.ok()) {
    throw new Error(`创建题目失败：${response.status()} ${await response.text()}`);
  }
  return (await response.json()) as CreatedQuestion;
}

export async function publishQuestion(api: APIRequestContext, id: string): Promise<void> {
  const response = await api.patch(`/api/v1/admin/questions/${id}/status`, {
    data: { status: "published" },
  });
  if (!response.ok()) {
    throw new Error(`发布题目失败：${response.status()} ${await response.text()}`);
  }
}

export async function findQuestionsByTag(
  api: APIRequestContext,
  tag: string,
): Promise<CreatedQuestion[]> {
  const response = await api.get(`/api/v1/questions?tag=${encodeURIComponent(tag)}&page=1&page_size=100`);
  if (!response.ok()) return [];
  const body = (await response.json()) as { items: CreatedQuestion[] };
  return body.items;
}

export async function cleanupQuestionsByTag(
  teacherApi: APIRequestContext,
  adminApi: APIRequestContext,
  tag: string,
): Promise<void> {
  for (const item of await findQuestionsByTag(adminApi, tag)) {
    if (item.status === "draft") {
      await teacherApi.delete(`/api/v1/questions/${item.id}`);
    } else if (item.status !== "archived") {
      await adminApi.patch(`/api/v1/admin/questions/${item.id}/status`, {
        data: { status: "archived" },
      });
    }
  }
}

export interface CreatedPaper {
  id: string;
  title: string;
  total_score: string;
}

export async function createPaper(
  api: APIRequestContext,
  input: {
    title: string;
    subject?: string;
    grade?: string;
    duration_minutes?: number;
    questions: Array<{ question_id: string; assigned_score: string }>;
  },
): Promise<CreatedPaper> {
  const response = await api.post("/api/v1/papers", {
    data: {
      subject: "数学",
      grade: "七年级",
      duration_minutes: 60,
      ...input,
    },
  });
  if (!response.ok()) {
    throw new Error(`创建试卷失败：${response.status()} ${await response.text()}`);
  }
  return (await response.json()) as CreatedPaper;
}

export async function removePaper(api: APIRequestContext, id: string | null): Promise<void> {
  if (!id) return;
  await api.delete(`/api/v1/papers/${id}`);
}

export async function registerTeacherAccount(
  schoolCode: string,
  email: string,
  password: string,
): Promise<void> {
  const api = await apiContext();
  const response = await api.post("/api/v1/auth/register", {
    data: {
      school_code: schoolCode,
      name: "E2E 专用教师",
      subject: "数学",
      email,
      password,
    },
  });
  const body = await response.text();
  await api.dispose();
  if (!response.ok()) {
    throw new Error(`注册教师失败：${response.status()} ${body}`);
  }
}

export async function loginStatus(email: string, password: string): Promise<number> {
  const api = await apiContext();
  const response = await api.post("/api/v1/auth/login", { data: { email, password } });
  await api.dispose();
  return response.status();
}
