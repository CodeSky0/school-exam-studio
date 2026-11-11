import { afterEach, describe, expect, it, vi } from "vitest";
import { updateQuestionStatus } from "./questions";

const question = {
  id: "e2e01f6c-b5b2-4d0a-9f4f-cfd5e518a7a8",
  creator_id: "5f6876f7-88a8-46e8-a20e-e6bcb75b6a01",
  creator_name: "周老师",
  subject: "物理",
  grade: "八年级",
  question_type: "true_false",
  difficulty: 2,
  tags: ["力学"],
  content: "<p>力的单位是牛顿。</p>",
  options: [],
  answer: "true",
  explanation: "",
  status: "published",
  created_at: "2026-11-01T08:00:00Z",
  updated_at: "2026-11-10T08:00:00Z",
};

afterEach(() => vi.unstubAllGlobals());

describe("管理员题目状态 API", () => {
  it("向既有审核端点提交发布状态并校验题目响应", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(question), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(updateQuestionStatus(question.id, "published")).resolves.toMatchObject({ status: "published" });
    expect(fetchMock).toHaveBeenCalledWith(`http://localhost:8080/api/v1/admin/questions/${question.id}/status`, expect.objectContaining({ method: "PATCH", body: JSON.stringify({ status: "published" }) }));
  });
});
