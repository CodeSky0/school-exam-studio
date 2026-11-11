import { afterEach, describe, expect, it, vi } from "vitest";
import { changePassword } from "./auth";
import { changePasswordSchema } from "./schemas";

afterEach(() => vi.unstubAllGlobals());

describe("修改密码", () => {
  it("校验新密码确认及与原密码不同", () => {
    expect(changePasswordSchema.safeParse({ old_password: "OldPassword123", new_password: "OldPassword123", confirm_password: "OldPassword123" }).success).toBe(false);
    expect(changePasswordSchema.safeParse({ old_password: "OldPassword123", new_password: "NewPassword123", confirm_password: "different" }).success).toBe(false);
    expect(changePasswordSchema.parse({ old_password: "OldPassword123", new_password: "NewPassword123", confirm_password: "NewPassword123" }).new_password).toBe("NewPassword123");
  });

  it("只提交 old_password 和 new_password", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: "ok" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(changePassword({ old_password: "OldPassword123", new_password: "NewPassword123" })).resolves.toEqual({ status: "ok" });
    expect(fetchMock).toHaveBeenCalledWith("http://localhost:8080/api/v1/auth/change-password", expect.objectContaining({
      method: "POST",
      body: JSON.stringify({ old_password: "OldPassword123", new_password: "NewPassword123" }),
      credentials: "include",
    }));
  });
});
