import { describe, expect, it } from "vitest";
import { richTextHasContent, sanitizeRichText } from "./rich-text";

describe("受限富文本", () => {
  it("保留基础排版与前后端两种公式属性", () => {
    const input = '<p><strong>已知</strong> <span data-type="inline-math" data-latex="x^2"></span></p>';
    expect(sanitizeRichText(input)).toBe(input);
    const stored = '<p><span data-type="inlineMath" data-latex="x^2"></span></p><div data-type="blockMath" data-latex="y^2"></div>';
    expect(sanitizeRichText(stored)).toBe(stored);
  });

  it("移除脚本、图片、事件属性和危险链接", () => {
    const input = '<p onclick="alert(1)">文本<script>alert(1)</script><img src=x onerror=alert(1)><a href="javascript:alert(1)">链接</a></p>';
    const result = sanitizeRichText(input);
    expect(result).not.toContain("script");
    expect(result).not.toContain("img");
    expect(result).not.toContain("onclick");
    expect(result).not.toContain("javascript:");
    expect(result).toContain("文本");
  });

  it("保留服务端允许的表格、上下标与代码标签", () => {
    const input = '<table><tbody><tr><td colspan="2">答案<sub>1</sub></td></tr></tbody></table><h4>标题</h4><pre><code>x = 1</code></pre>';
    expect(sanitizeRichText(input)).toBe(input);
  });

  it("公式节点本身视为有效内容，空段落不算内容", () => {
    expect(richTextHasContent('<p><span data-type="inline-math" data-latex="\\frac{1}{2}"></span></p>')).toBe(true);
    expect(richTextHasContent("<p><br></p>")).toBe(false);
  });

  it("把文本公式转换为题库统一的 data-latex 节点", () => {
    const inline = sanitizeRichText("<p>已知 $x^2$ 求值</p>");
    expect(inline).toBe('<p>已知 <span data-type="inlineMath" data-latex="x^2"></span> 求值</p>');

    const block = sanitizeRichText("<p>$$\\frac{1}{2}$$</p>");
    expect(block).toContain('data-type="blockMath"');
    expect(block).toContain('data-latex="\\frac{1}{2}"');

    const bracket = sanitizeRichText("<p>求 \\(a+b\\) 的和</p>");
    expect(bracket).toContain('data-type="inlineMath"');
    expect(bracket).toContain('data-latex="a+b"');
  });

  it("代码中的美元符号保持原样，未闭合分隔符不转换", () => {
    const code = sanitizeRichText("<p><code>$x^2$</code></p>");
    expect(code).toBe("<p><code>$x^2$</code></p>");
    const unclosed = sanitizeRichText("<p>价格 $5 未闭合</p>");
    expect(unclosed).toBe("<p>价格 $5 未闭合</p>");
  });
});
