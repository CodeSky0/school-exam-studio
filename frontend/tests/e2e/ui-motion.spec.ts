import { test, expect } from "@playwright/test";
import { adminStorage, TEACHER_EMAIL, TEACHER_PASSWORD } from "./helpers";

async function expectNoHorizontalOverflow(page: import("@playwright/test").Page) {
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
  );
  expect(overflow, "根节点不应出现横向溢出").toBe(false);
}

test.describe("移动导航与键盘可访问性", () => {
  test.use({ storageState: adminStorage, viewport: { width: 390, height: 844 } });

  test("移动导航打开后聚焦关闭按钮，Escape 归还菜单按钮焦点", async ({ page }) => {
    await page.goto("/admin/dashboard");
    const trigger = page.getByRole("button", { name: "打开导航" });
    await trigger.click();
    const dialog = page.getByRole("dialog", { name: "主导航" });
    await expect(dialog).toBeVisible();
    await expect(dialog.locator("aside").getByRole("button", { name: "关闭导航" })).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
    await expectNoHorizontalOverflow(page);
  });

  test("首个 Tab 聚焦跳到主内容链接，Enter 后主内容获得焦点", async ({ page }) => {
    await page.goto("/admin/dashboard");
    await expect(page.getByRole("heading", { name: "学校总览" })).toBeVisible();
    await page.keyboard.press("Tab");
    const skipLink = page.getByRole("link", { name: "跳到主内容" });
    await expect(skipLink).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.locator("#main-content")).toBeFocused();
  });
});

test.describe("公共页面响应式", () => {
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    test(`首页在 ${viewport.width}px 视口无横向溢出`, async ({ page }) => {
      await page.setViewportSize(viewport);
      await page.goto("/");
      await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
      await expectNoHorizontalOverflow(page);
    });
  }

  for (const path of ["/login", "/register"]) {
    test(`${path} 只有一个 h1 且所有输入框有可见标签`, async ({ page }) => {
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(path);
      await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
      const unlabeled = await page.evaluate(() => {
        return Array.from(document.querySelectorAll("input"))
          .filter((input) => {
            const label = input.id ? document.querySelector(`label[for="${input.id}"]`) : null;
            const wrapped = input.closest("label");
            const ariaLabel = input.getAttribute("aria-label");
            const visible = (element: Element | null) =>
              Boolean(element && element.getClientRects().length > 0);
            return !(visible(label) || visible(wrapped) || ariaLabel);
          })
          .map((input) => input.getAttribute("name") ?? input.type);
      });
      expect(unlabeled, "所有输入框必须有可见标签").toEqual([]);
      await expectNoHorizontalOverflow(page);
    });
  }

  test("已初始化环境访问 /init 跳转登录页", async ({ page }) => {
    await page.goto("/init");
    await expect(page).toHaveURL(/\/login$/);
  });

  test("reduced-motion 下登录表单仍可提交", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/login");
    await page.getByLabel("邮箱").fill(TEACHER_EMAIL);
    await page.getByLabel("密码").fill(TEACHER_PASSWORD);
    await page.getByRole("button", { name: "登录" }).click();
    await expect(page).toHaveURL(/\/teacher\/dashboard/, { timeout: 20_000 });
  });
});
