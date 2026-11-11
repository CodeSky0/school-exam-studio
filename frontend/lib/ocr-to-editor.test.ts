import { describe, expect, it } from "vitest";
import { ocrToEditorJSON } from "./ocr-to-editor";

describe("OCR 到编辑器 JSON 转换", () => {
  it("保留中文并转换行内公式", () => {
    const result = ocrToEditorJSON("已知三角形两直角边为 $a=3$ 与 $b=4$，求斜边。");
    expect(result).toEqual({
      type: "doc",
      content: [{
        type: "paragraph",
        content: [
          { type: "text", text: "已知三角形两直角边为 " },
          { type: "inlineMath", attrs: { latex: "a=3" } },
          { type: "text", text: " 与 " },
          { type: "inlineMath", attrs: { latex: "b=4" } },
          { type: "text", text: "，求斜边。" },
        ],
      }],
    });
  });

  it("转换可跨行的独立公式并保留普通换行", () => {
    const result = ocrToEditorJSON("解方程：\n$$\nx^2-4=0\n$$\n写出正根");
    expect(JSON.stringify(result)).toContain('"type":"blockMath"');
    expect(JSON.stringify(result)).toContain('"latex":"x^2-4=0"');
    expect(JSON.stringify(result)).toContain('"type":"hardBreak"');
    expect(JSON.stringify(result)).toContain("写出正根");
  });

  it("把原始 HTML 当作纯文本而不是节点", () => {
    const source = '已知 <img src=x onerror=alert(1)> <script>alert("x")</script> $x^2=4$';
    const result = ocrToEditorJSON(source);
    const serialized = JSON.stringify(result);
    expect(serialized).toContain('<img src=x onerror=alert(1)> <script>alert(\\"x\\")</script>');
    expect(serialized).toContain('"type":"inlineMath"');
    expect(serialized).not.toContain('"type":"image"');
    expect(serialized).not.toContain('"type":"script"');
  });

  it("按空行划分段落并保留未闭合定界符", () => {
    const result = ocrToEditorJSON("第一题 $x+1\n\n第二题");
    expect(result.content).toHaveLength(2);
    expect(JSON.stringify(result)).toContain("第一题 $x+1");
    expect(JSON.stringify(result)).toContain("第二题");
  });

  it("空输入返回可编辑的空文档", () => {
    expect(ocrToEditorJSON("  \n ")).toEqual({ type: "doc", content: [{ type: "paragraph" }] });
  });
});
