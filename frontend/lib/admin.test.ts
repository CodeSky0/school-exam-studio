import { afterEach, describe, expect, it, vi } from "vitest";
import { adminStatsSchema, teacherStatsSchema } from "./stats";
import { getSchool, listTeachers, resetTeacherPassword, serializeTeacherFilters, updateSchoolName, updateTeacherStatus } from "./admin";

const user = {
  id: "e2e01f6c-b5b2-4d0a-9f4f-cfd5e518a7a8",
  school_id: "5f6876f7-88a8-46e8-a20e-e6bcb75b6a01",
  name: "周老师",
  subject: "物理",
  email: "teacher@example.com",
  role: "teacher",
  status: "active",
  created_at: "2026-11-10T08:00:00Z",
  updated_at: "2026-11-10T08:00:00Z",
};

const school = {
  id: user.school_id,
  name: "明德实验中学",
  school_code: "01234567",
  created_at: "2026-11-01T08:00:00Z",
  updated_at: "2026-11-10T08:00:00Z",
};

afterEach(() => vi.unstubAllGlobals());

describe("管理端筛选与响应校验", () => {
  it("序列化学科、状态与分页并修剪学科", () => {
    expect(serializeTeacherFilters({ subject: " 数学 ", status: "disabled", page: 2, page_size: 20 }).toString())
      .toBe("subject=%E6%95%B0%E5%AD%A6&status=disabled&page=2&page_size=20");
    expect(serializeTeacherFilters({ subject: " ", page: 1, page_size: 10 }).toString()).toBe("page=1&page_size=10");
  });

  it("拒绝负数统计", () => {
    expect(adminStatsSchema.safeParse({ teachers: { active: -1, disabled: 0 }, questions: { draft: 0, published: 0, archived: 0 }, papers: 0 }).success).toBe(false);
    expect(teacherStatsSchema.parse({ questions: { draft: 2, published: 3, archived: 1 }, papers: 4 }).papers).toBe(4);
  });
});

describe("管理端 API", () => {
  it("读取教师列表并校验用户字段", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [user], total: 1, page: 1, page_size: 10 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(listTeachers({ page: 1, page_size: 10 })).resolves.toMatchObject({ total: 1 });
    expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8080/api/v1/admin/teachers?page=1&page_size=10");
  });

  it("状态修改只发送约定字段", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...user, status: "disabled" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await updateTeacherStatus(user.id, "disabled");
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ method: "PATCH", body: JSON.stringify({ status: "disabled" }) });
  });

  it("校验一次性密码并请求重置", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ temporary_password: "r4ndom-Temporary-Password" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(resetTeacherPassword(user.id)).resolves.toEqual({ temporary_password: "r4ndom-Temporary-Password" });
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ method: "POST" });
  });

  it("学校名称更新只发送 name", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(school), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await getSchool();
    await updateSchoolName("明德实验中学");
    expect(fetchMock.mock.calls[1][0]).toBe("http://localhost:8080/api/v1/admin/school");
    expect(fetchMock.mock.calls[1][1]).toMatchObject({ method: "PATCH", body: JSON.stringify({ name: "明德实验中学" }) });
  });
});
