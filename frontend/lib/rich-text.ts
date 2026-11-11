import sanitizeHtml from "sanitize-html";

const allowedTags = [
  "p", "br", "strong", "em", "u", "s", "ul", "ol", "li", "blockquote", "h2", "h3", "span", "div",
];

export function sanitizeRichText(value: string): string {
  return sanitizeHtml(value, {
    allowedTags,
    allowedAttributes: {
      span: ["data-type", "data-latex"],
      div: ["data-type", "data-latex"],
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
