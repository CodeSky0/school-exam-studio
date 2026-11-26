import { test, expect, type Page } from "@playwright/test";
import { adminStorage, apiContext, cleanupQuestionsByTag, createQuestion, teacherStorage, uniqueLabel } from "./helpers";

function richTextEditor(page: Page, label: string) {
  return page.locator(`[contenteditable="true"][aria-label="${label}"]`);
}

test.describe("题目创建与审核", () => {
  test.use({ storageState: teacherStorage });

  test("教师创建含公式草稿，管理员发布后教师可见", async ({ browser, page }) => {
    const marker = uniqueLabel("求方程");
    const teacherApi = await apiContext(teacherStorage);
    const adminApi = await apiContext(adminStorage);

    try {
      await page.goto("/teacher/questions");
      await page.getByRole("button", { name: "新建题目" }).click();
      await page.locator("#question-grade").fill("七年级");
      await page.locator("#question-tags").fill(marker);
      await richTextEditor(page, "题干").fill(`${marker} $x^2=4$ 的正根`);
      await richTextEditor(page, "选项 A").fill("2");
      await richTextEditor(page, "选项 B").fill("-2");
      await page.getByRole("radio", { name: "选项 A" }).check();
      await page.getByRole("button", { name: "保存草稿" }).first().click();
      await expect(page.getByText("新草稿已保存")).toBeVisible();
      await expect(page.getByRole("row", { name: new RegExp(marker) })).toContainText("草稿");

      const adminContext = await browser.newContext({ storageState: adminStorage });
      const adminPage = await adminContext.newPage();
      await adminPage.goto("/admin/questions");
      const draftRow = adminPage.getByRole("row", { name: new RegExp(marker) });
      await draftRow.getByRole("button", { name: "发布" }).click();
      await adminPage.getByRole("button", { name: "确认发布" }).click();
      await expect(adminPage.getByText("题目已发布")).toBeVisible();
      await adminContext.close();

      await page.reload();
      await expect(page.getByRole("row", { name: new RegExp(marker) })).toContainText("已发布");
    } finally {
      await cleanupQuestionsByTag(teacherApi, adminApi, marker);
      await teacherApi.dispose();
      await adminApi.dispose();
    }
  });

  test("教师可按难度组合筛选自己的题目", async ({ page }) => {
    const marker = uniqueLabel("筛选");
    const teacherApi = await apiContext(teacherStorage);
    const adminApi = await apiContext(adminStorage);

    try {
      await page.goto("/teacher/questions");
      await page.getByRole("button", { name: "新建题目" }).click();
      await page.locator("#question-grade").fill("八年级");
      await page.locator("#question-tags").fill(marker);
      await page.locator("#question-difficulty").selectOption("5");
      await richTextEditor(page, "题干").fill(`${marker} 高难度题目`);
      await richTextEditor(page, "选项 A").fill("正确选项");
      await richTextEditor(page, "选项 B").fill("干扰选项");
      await page.getByRole("radio", { name: "选项 A" }).check();
      await page.getByRole("button", { name: "保存草稿" }).first().click();
      await expect(page.getByText("新草稿已保存")).toBeVisible();

      await page.locator("#filter-difficulty").selectOption("5");
      await page.getByRole("button", { name: "应用筛选" }).click();
      const row = page.getByRole("row", { name: new RegExp(marker) });
      await expect(row).toBeVisible();
      await expect(row).toContainText("5 / 5");
    } finally {
      await cleanupQuestionsByTag(teacherApi, adminApi, marker);
      await teacherApi.dispose();
      await adminApi.dispose();
    }
  });

  test("390px 题目摘要可展开答案解析，操作可见且筛选焦点稳定", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const marker = uniqueLabel("移动");
    const teacherApi = await apiContext(teacherStorage);
    const adminApi = await apiContext(adminStorage);

    try {
      await createQuestion(teacherApi, {
        tags: [marker],
        content: `<p>${marker} 求方程 $x^2=4$ 的正根</p>`,
      });

      await page.goto("/teacher/questions");
      await page.locator("#filter-tag").fill(marker);
      const applyButton = page.getByRole("button", { name: "应用筛选" });
      await applyButton.click();

      const card = page.locator("article", { hasText: marker });
      await expect(card).toBeVisible();
      await expect(card.getByRole("button", { name: "编辑" })).toBeVisible();
      await expect(card.getByRole("button", { name: "删除" })).toBeVisible();
      await expect(card).toContainText("数学 · 七年级");
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(overflow, "移动题目列表不应横向溢出").toBe(false);
      await expect(applyButton).toBeFocused();

      const toggle = card.getByRole("button", { name: "查看详情" });
      await toggle.click();
      await expect(card.getByRole("heading", { name: "答案", exact: true })).toBeVisible();
      await expect(card.getByRole("heading", { name: "解析", exact: true })).toBeVisible();
      await expect(card.getByRole("button", { name: "收起详情" })).toBeFocused();
      await card.getByRole("button", { name: "收起详情" }).click();
      await expect(card.getByRole("heading", { name: "答案", exact: true })).toBeHidden();
    } finally {
      await cleanupQuestionsByTag(teacherApi, adminApi, marker);
      await teacherApi.dispose();
      await adminApi.dispose();
    }
  });
});
