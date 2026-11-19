import sanitizeHtml from "sanitize-html";

const allowedTags = [
  "p", "br", "hr", "strong", "b", "em", "i", "u", "s", "del", "sub", "sup",
  "code", "pre", "ul", "ol", "li", "blockquote",
  "h1", "h2", "h3", "h4", "h5", "h6", "span", "div",
  "table", "caption", "colgroup", "col", "thead", "tbody", "tfoot", "tr", "th", "td",
];

function isEscaped(value: string, index: number) {
  let backslashes = 0;
  for (let cursor = index - 1; cursor >= 0 && value[cursor] === "\\"; cursor -= 1) backslashes += 1;
  return backslashes % 2 === 1;
}

function unescapedDelimiterIndex(value: string, delimiter: string, start: number) {
  let cursor = start;
  while (cursor <= value.length - delimiter.length) {
    const position = value.indexOf(delimiter, cursor);
    if (position < 0) return -1;
    if (!isEscaped(value, position)) return position;
    cursor = position + delimiter.length;
  }
  return -1;
}

function mathMarkup(latex: string, block: boolean) {
  const tag = block ? "div" : "span";
  const type = block ? "blockMath" : "inlineMath";
  const escaped = latex
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
  return `<${tag} data-type="${type}" data-latex="${escaped}"></${tag}>`;
}

// convertTextMath 把纯文本中的 LaTeX 公式转换为题库统一的 data-latex 节点。
function convertTextMath(text: string): string {
  let output = "";
  let index = 0;
  while (index < text.length) {
    const char = text[index];
    if (char === "\\" && !isEscaped(text, index) && (text[index + 1] === "(" || text[index + 1] === "[")) {
      const closing = text[index + 1] === "(" ? "\\)" : "\\]";
      const end = unescapedDelimiterIndex(text, closing, index + 2);
      if (end < 0) {
        output += char;
        index += 1;
        continue;
      }
      const latex = text.slice(index + 2, end).trim();
      if (!latex) {
        output += text.slice(index, end + closing.length);
        index = end + closing.length;
        continue;
      }
      output += mathMarkup(latex, text[index + 1] === "[");
      index = end + closing.length;
      continue;
    }
    if (char === "$" && !isEscaped(text, index)) {
      const block = text.startsWith("$$", index);
      const delimiter = block ? "$$" : "$";
      const contentStart = index + delimiter.length;
      const end = unescapedDelimiterIndex(text, delimiter, contentStart);
      if (end < 0) {
        output += text.slice(index);
        break;
      }
      const latex = text.slice(contentStart, end);
      if (!latex.trim() || (!block && /[\r\n]/.test(latex))) {
        output += text.slice(index, end + delimiter.length);
        index = end + delimiter.length;
        continue;
      }
      output += mathMarkup(latex, block);
      index = end + delimiter.length;
      continue;
    }
    output += char;
    index += 1;
  }
  return output;
}

// expandTextMath 兼容导入文档、AI 生成内容里的 $...$、$$...$$、\(...\)、\[...\] 公式。
export function expandTextMath(value: string): string {
  if (!value.includes("$") && !value.includes("\\(") && !value.includes("\\[")) return value;
  let skipDepth = 0;
  return value
    .split(/(<[^>]*>)/g)
    .map((part) => {
      if (part.startsWith("<")) {
        const match = /^<\s*(\/?)\s*(code|pre)\b/i.exec(part);
        if (match) {
          if (match[1]) skipDepth = Math.max(0, skipDepth - 1);
          else if (!part.endsWith("/>")) skipDepth += 1;
        }
        return part;
      }
      return skipDepth > 0 ? part : convertTextMath(part);
    })
    .join("");
}

export function sanitizeRichText(value: string): string {
  return sanitizeHtml(expandTextMath(value), {
    allowedTags,
    allowedAttributes: {
      span: ["data-type", "data-latex"],
      div: ["data-type", "data-latex"],
      th: ["colspan", "rowspan"],
      td: ["colspan", "rowspan"],
    },
    allowedSchemes: [],
    allowProtocolRelative: false,
    transformTags: {
      span: (_tagName, attribs) => {
        if (attribs["data-type"] === "inline-math" || attribs["data-type"] === "inlineMath") {
          return { tagName: "span", attribs: { "data-type": attribs["data-type"], "data-latex": attribs["data-latex"] ?? "" } };
        }
        return { tagName: "span", attribs: {} as Record<string, string> };
      },
      div: (_tagName, attribs) => {
        if (attribs["data-type"] === "block-math" || attribs["data-type"] === "blockMath") {
          return { tagName: "div", attribs: { "data-type": attribs["data-type"], "data-latex": attribs["data-latex"] ?? "" } };
        }
        return { tagName: "div", attribs: {} as Record<string, string> };
      },
    },
  });
}

export function richTextHasContent(value: string): boolean {
  const sanitized = sanitizeRichText(value);
  if (/data-latex="[^\"]+"/.test(sanitized)) return true;
  const text = sanitized
    .replace(/<[^>]*>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#39);/g, " ")
    .trim();
  return text.length > 0;
}
