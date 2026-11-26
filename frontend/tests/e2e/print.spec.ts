import fs from "node:fs";
import { test, expect } from "@playwright/test";
import {
  adminStorage,
  apiContext,
  cleanupQuestionsByTag,
  createPaper,
  createQuestion,
  publishQuestion,
  removePaper,
  teacherStorage,
  uniqueLabel,
} from "./helpers";

test.describe("A4 打印", () => {
  test.use({ storageState: teacherStorage });

  test("打印页只保留试卷内容并生成 A4 尺寸 PDF", async ({ page }, testInfo) => {
    const marker = uniqueLabel("打印");
    const teacherApi = await apiContext(teacherStorage);
    const adminApi = await apiContext(adminStorage);
    let paperId: string | null = null;

    try {
      const first = await createQuestion(teacherApi, {
        tags: [marker],
        content: `<p>${marker} 第一题，求方程 $x^2=4$ 的正根。</p>`,
      });
      const second = await createQuestion(teacherApi, {
        tags: [marker],
        question_type: "fill_blank",
        options: [],
        answer: "<p>2</p>",
        content: `<p>${marker} 第二题，计算 $0.10+0.20$ 的结果。</p>`,
      });
      await publishQuestion(adminApi, first.id);
      await publishQuestion(adminApi, second.id);
      const paper = await createPaper(teacherApi, {
        title: `${marker} 数学测验`,
        duration_minutes: 45,
        questions: [
          { question_id: first.id, assigned_score: "0.10" },
          { question_id: second.id, assigned_score: "0.20" },
        ],
      });
      paperId = paper.id;
      expect(paper.total_score).toBe("0.30");

      await page.goto(`/teacher/papers/${paper.id}/print`);
      const sheet = page.locator(".paper-sheet");
      await expect(sheet).toBeVisible();
      await expect(sheet).toContainText("E2E 测试学校");
      await expect(sheet).toContainText(paper.title);
      await expect(sheet).toContainText("0.30");
      await expect(page.locator(".paper-seal")).toBeVisible();
      await expect(page.locator(".paper-question")).toHaveCount(2);

      const screenRatio = await sheet.evaluate((element) => {
        const rect = element.getBoundingClientRect();
        return rect.width / rect.height;
      });
      expect(screenRatio, "屏幕预览应保持 A4 纸张比例").toBeCloseTo(210 / 297, 1);

      const pageSize = await page.evaluate(() => {
        const pageRules: string[] = [];
        for (const styleSheet of Array.from(document.styleSheets)) {
          let rules: CSSRuleList;
          try {
            rules = styleSheet.cssRules;
          } catch {
            continue;
          }
          for (const rule of Array.from(rules)) {
            if (rule instanceof CSSPageRule) pageRules.push(rule.cssText);
          }
        }
        return pageRules.join("\n");
      });
      expect(pageSize).toMatch(/size:\s*a4/i);

      await page.emulateMedia({ media: "print" });
      await expect(page.locator(".print-toolbar")).toBeHidden();
      await expect(sheet).toBeVisible();

      const pdf = await page.pdf({ preferCSSPageSize: true, printBackground: true });
      const pdfPath = testInfo.outputPath("paper-a4.pdf");
      fs.writeFileSync(pdfPath, pdf);
      const mediaBox = pdf
        .toString("latin1")
        .match(/\/MediaBox\s*\[\s*0\s+0\s+([\d.]+)\s+([\d.]+)\s*\]/);
      expect(mediaBox, "PDF 应包含 MediaBox 页面尺寸").not.toBeNull();
      const width = Number(mediaBox?.[1]);
      const height = Number(mediaBox?.[2]);
      expect(width).toBeGreaterThan(594);
      expect(width).toBeLessThan(596);
      expect(height).toBeGreaterThan(840);
      expect(height).toBeLessThan(843);
    } finally {
      await removePaper(teacherApi, paperId);
      await cleanupQuestionsByTag(teacherApi, adminApi, marker);
      await teacherApi.dispose();
      await adminApi.dispose();
    }
  });

  test("390px 屏幕预览不压缩纸张，舞台可横向滚动且根节点不溢出", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const marker = uniqueLabel("移动打印");
    const teacherApi = await apiContext(teacherStorage);
    const adminApi = await apiContext(adminStorage);
    let paperId: string | null = null;

    try {
      const question = await createQuestion(teacherApi, {
        tags: [marker],
        content: `<p>${marker} 计算 $0.10+0.20$ 的结果。</p>`,
        question_type: "fill_blank",
        options: [],
        answer: "<p>0.30</p>",
      });
      await publishQuestion(adminApi, question.id);
      const paper = await createPaper(teacherApi, {
        title: `${marker} 移动预览`,
        questions: [{ question_id: question.id, assigned_score: "0.30" }],
      });
      paperId = paper.id;

      await page.goto(`/teacher/papers/${paper.id}/print`);
      const stage = page.locator(".paper-preview-stage");
      const sheet = page.locator(".paper-sheet");
      await expect(sheet).toBeVisible();

      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(overflow, "移动打印页根节点不应横向溢出").toBe(false);

      const stageScrollable = await stage.evaluate(
        (element) => element.scrollWidth > element.clientWidth,
      );
      expect(stageScrollable, "预览舞台应可横向滚动").toBe(true);

      const ratio = await sheet.evaluate((element) => {
        const rect = element.getBoundingClientRect();
        return rect.width / rect.height;
      });
      expect(ratio, "窄屏下纸张仍保持完整 A4 比例").toBeCloseTo(210 / 297, 1);
    } finally {
      await removePaper(teacherApi, paperId);
      await cleanupQuestionsByTag(teacherApi, adminApi, marker);
      await teacherApi.dispose();
      await adminApi.dispose();
    }
  });
});
