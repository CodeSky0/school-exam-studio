import { test, expect } from "@playwright/test";
import { TEACHER_EMAIL, TEACHER_PASSWORD, readMeta, uniqueLabel } from "./helpers";

test("未登录访问教师题库会被重定向到登录页", async ({ page }) => {
  await page.goto("/teacher/questions");
  await expect(page).toHaveURL(/\/login$/);
});

test("教师通过登录页登录并退出", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("邮箱").fill(TEACHER_EMAIL);
  await page.getByLabel("密码").fill(TEACHER_PASSWORD);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/$/);

  await page.goto("/teacher/questions");
  await expect(page.getByRole("heading", { name: "题库", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "退出" }).click();

  await page.goto("/teacher/questions");
  await expect(page).toHaveURL(/\/login$/);
});

test("访客使用学校代码注册教师并登录", async ({ page }) => {
  const meta = readMeta();
  test.skip(!meta.schoolCode, "未记录学校代码，无法执行注册流程");
  const email = `${uniqueLabel("register")}@example.com`;
  const password = "RegisterE2E!2026Secure";

  await page.goto("/register");
  await page.getByLabel("学校代码").fill(meta.schoolCode as string);
  await page.getByLabel("姓名").fill("注册测试教师");
  await page.getByLabel("学科").fill("数学");
  await page.getByLabel("邮箱").fill(email);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "创建教师账号" }).click();
  await expect(page.getByText("教师账号已创建")).toBeVisible();

  await page.getByRole("link", { name: "前往登录" }).click();
  await page.getByLabel("邮箱").fill(email);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/$/);
});
