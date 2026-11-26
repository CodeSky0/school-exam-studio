import { test, expect, type Page } from "@playwright/test";
import {
  adminStorage,
  apiContext,
  cleanupQuestionsByTag,
  createQuestion,
  publishQuestion,
  removePaper,
  teacherStorage,
  uniqueLabel,
} from "./helpers";

async function expectCartFocusFlow(page: Page, marker: string) {
  const teacherApi = await apiContext(teacherStorage);
  const adminApi = await apiContext(adminStorage);
  try {
    const first = await createQuestion(teacherApi, {
      tags: [marker],
      content: `<p>${marker} 第一题 $0.10$</p>`,
    });
    const second = await createQuestion(teacherApi, {
      tags: [marker],
      content: `<p>${marker} 第二题 $0.20$</p>`,
    });
    await publishQuestion(adminApi, first.id);
    await publishQuestion(adminApi, second.id);

    await page.goto("/teacher/papers");
    await page.locator("#picker-tag").fill(marker);
    await page.getByRole("button", { name: "应用筛选" }).click();
    const picker = page.locator("section", {
      has: page.getByRole("heading", { name: "选择题库题目" }),
    });
    const addButtons = picker.getByRole("button", { name: "加入试卷" });
    await expect(addButtons).toHaveCount(2);
    await addButtons.nth(0).click();
    await addButtons.nth(0).click();

    const cart = page.locator("section", {
      has: page.getByRole("heading", { name: "卷面题目" }),
    });
    const items = cart.getByRole("listitem");
    await expect(items).toHaveCount(2);
    const firstItemId = (await items.nth(0).getAttribute("id")) as string;
    const secondItemId = (await items.nth(1).getAttribute("id")) as string;
    const firstQuestionId = firstItemId.replace("cart-question-", "");
    const secondQuestionId = secondItemId.replace("cart-question-", "");

    await page.getByRole("button", { name: "下移第 1 题" }).click();
    await expect(items.nth(0)).toHaveId(secondItemId);
    await expect(items.nth(1)).toHaveId(firstItemId);
    await expect(page.locator(`#cart-move-down-${firstQuestionId}`)).toBeFocused();
    await expect(cart.locator("p[role='status']").first()).toContainText("已将第 1 题下移至第 2 题");

    await page.getByRole("button", { name: "上移第 2 题" }).click();
    await expect(items.nth(0)).toHaveId(firstItemId);
    await expect(items.nth(1)).toHaveId(secondItemId);
    await expect(page.locator(`#cart-move-up-${firstQuestionId}`)).toBeFocused();

    await page.getByRole("button", { name: "删除第 1 题" }).click();
    await expect(items).toHaveCount(1);
    await expect(page.locator(`#${secondItemId}`)).toBeFocused();

    await page.getByRole("button", { name: "删除第 1 题" }).click();
    await expect(items).toHaveCount(0);
    await expect(page.locator("#paper-cart-title")).toBeFocused();
  } finally {
    await cleanupQuestionsByTag(teacherApi, adminApi, marker);
    await teacherApi.dispose();
    await adminApi.dispose();
  }
}

test.describe("组卷与分值", () => {
  test.use({ storageState: teacherStorage });

  test("选题、排序、整数百分总分、刷新恢复并保存试卷", async ({ page }) => {
    const marker = uniqueLabel("组卷");
    const teacherApi = await apiContext(teacherStorage);
    const adminApi = await apiContext(adminStorage);
    let paperId: string | null = null;

    try {
      const first = await createQuestion(teacherApi, {
        tags: [marker],
        content: `<p>${marker} 第一题 $0.10$</p>`,
      });
      const second = await createQuestion(teacherApi, {
        tags: [marker],
        content: `<p>${marker} 第二题 $0.20$</p>`,
      });
      await publishQuestion(adminApi, first.id);
      await publishQuestion(adminApi, second.id);

      await page.goto("/teacher/papers");
      await page.locator("#picker-tag").fill(marker);
      await page.getByRole("button", { name: "应用筛选" }).click();
      const picker = page.locator("section", {
        has: page.getByRole("heading", { name: "选择题库题目" }),
      });
      const addButtons = picker.getByRole("button", { name: "加入试卷" });
      await expect(addButtons).toHaveCount(2);
      await addButtons.nth(0).click();
      await addButtons.nth(0).click();

      const cart = page.locator("section", {
        has: page.getByRole("heading", { name: "卷面题目" }),
      });
      await expect(cart.getByRole("listitem")).toHaveCount(2);

      await page.reload();
      await expect(cart.getByRole("listitem")).toHaveCount(2);

      await page.getByRole("button", { name: "下移第 1 题" }).click();
      await page.getByLabel("本题分值").nth(0).fill("0.10");
      await page.getByLabel("本题分值").nth(1).fill("0.20");
      await expect(cart).toContainText("0.30");

      await page.locator("#paper-title").fill(`${marker} 数学小测`);
      await page.locator("#paper-grade").fill("七年级");
      await page.locator("#paper-duration").fill("60");
      await page.getByRole("button", { name: "创建试卷" }).click();
      await expect(page).toHaveURL(/\/teacher\/papers\/[0-9a-f-]+$/);
      paperId = page.url().split("/").pop() ?? null;

      await page.reload();
      await expect(page.locator("#paper-title")).toHaveValue(`${marker} 数学小测`);
      await expect(page.getByLabel("本题分值").nth(0)).toHaveValue("0.10");
      await expect(page.getByLabel("本题分值").nth(1)).toHaveValue("0.20");
      await expect(cart.getByRole("listitem")).toHaveCount(2);
    } finally {
      await removePaper(teacherApi, paperId);
      await cleanupQuestionsByTag(teacherApi, adminApi, marker);
      await teacherApi.dispose();
      await adminApi.dispose();
    }
  });

  test("购物车上移下移保持焦点与 DOM 顺序，删除后焦点落到相邻题目或标题", async ({ page }) => {
    await expectCartFocusFlow(page, uniqueLabel("焦点"));
  });

  test("reduced-motion 下购物车排序与删除焦点行为一致", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await expectCartFocusFlow(page, uniqueLabel("焦点降级"));
  });
});
