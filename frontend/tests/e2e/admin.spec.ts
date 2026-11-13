import { test, expect } from "@playwright/test";
import {
  adminStorage,
  apiContext,
  loginStatus,
  readMeta,
  registerTeacherAccount,
  uniqueLabel,
} from "./helpers";

test.describe("教师管理与学校设置", () => {
  test.use({ storageState: adminStorage });

  test("管理员禁用教师使旧会话失效，并可重新启用", async ({ page }) => {
    const meta = readMeta();
    test.skip(!meta.schoolCode, "未记录学校代码，无法注册专用教师");
    const email = `${uniqueLabel("disable")}@example.com`;
    const password = "DisableE2E!2026Secure";
    await registerTeacherAccount(meta.schoolCode as string, email, password);

    const teacherSession = await apiContext();
    const login = await teacherSession.post("/api/v1/auth/login", {
      data: { email, password },
    });
    expect(login.ok()).toBeTruthy();

    try {
      await page.goto("/admin/teachers");
      const row = page.getByRole("row", { name: new RegExp(email) });
      await row.getByRole("button", { name: "禁用" }).click();
      await page.getByRole("button", { name: "确认禁用" }).click();
      await expect(page.getByText("教师账号已禁用，旧会话将失效")).toBeVisible();
      await expect(row).toContainText("已禁用");

      const me = await teacherSession.get("/api/v1/auth/me");
      expect(me.status()).toBe(401);

      await row.getByRole("button", { name: "启用" }).click();
      await page.getByRole("button", { name: "确认启用" }).click();
      await expect(page.getByText("教师账号已启用")).toBeVisible();
      await expect(row).toContainText("正常");
    } finally {
      await teacherSession.dispose();
    }
  });

  test("管理员重置密码后旧密码失效，一次性临时密码可登录", async ({ page }) => {
    const meta = readMeta();
    test.skip(!meta.schoolCode, "未记录学校代码，无法注册专用教师");
    const email = `${uniqueLabel("reset")}@example.com`;
    const password = "ResetE2E!2026Secure";
    await registerTeacherAccount(meta.schoolCode as string, email, password);

    await page.goto("/admin/teachers");
    const row = page.getByRole("row", { name: new RegExp(email) });
    await row.getByRole("button", { name: "重置密码" }).click();
    await page.getByRole("button", { name: "确认重置" }).click();

    const temporaryInput = page.locator("#temporary-password");
    await expect(temporaryInput).toBeVisible();
    const temporaryPassword = await temporaryInput.inputValue();
    expect(temporaryPassword.length).toBeGreaterThanOrEqual(16);

    expect(await loginStatus(email, password)).toBe(401);
    expect(await loginStatus(email, temporaryPassword)).toBe(200);
  });

  test("管理员修改学校名称后可恢复", async ({ page }) => {
    await page.goto("/admin/settings");
    const nameInput = page.locator("#school-name");
    await expect(nameInput).toBeVisible();
    const original = await nameInput.inputValue();
    const updated = uniqueLabel("E2E 学校");

    await nameInput.fill(updated);
    await page.getByRole("button", { name: "保存学校名称" }).click();
    await expect(page.getByText("学校名称已保存")).toBeVisible();
    await page.reload();
    await expect(page.locator("#school-name")).toHaveValue(updated);

    await page.locator("#school-name").fill(original);
    await page.getByRole("button", { name: "保存学校名称" }).click();
    await expect(page.getByText("学校名称已保存")).toBeVisible();
  });
});
