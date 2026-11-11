import { describe, expect, it } from "vitest";
import { renderToHTMLString } from "@tiptap/static-renderer";
import { questionEditorExtensions } from "@/components/question/math-extension";

describe("公式存储序列化", () => {
  it("输出后端白名单使用的公式属性并兼容 OCR JSON", () => {
    const html = renderToHTMLString({
      content: {
        type: "doc",
        content: [
          { type: "paragraph", content: [{ type: "text", text: "已知 " }, { type: "inlineMath", attrs: { latex: "x^2" } }] },
          { type: "blockMath", attrs: { latex: "\\frac{1}{2}" } },
        ],
      },
      extensions: questionEditorExtensions,
    });
    expect(html).toContain('data-type="inlineMath"');
    expect(html).toContain('data-type="blockMath"');
    expect(html).toContain('data-latex="x^2"');
    expect(html).toContain('data-latex="\\frac{1}{2}"');
  });
});
