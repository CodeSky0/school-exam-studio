import type { JSONContent } from "@tiptap/core";

function textNode(text: string): JSONContent {
  return { type: "text", text };
}

function parseInlineText(text: string): JSONContent[] {
  const content: JSONContent[] = [];
  const pattern = /\$([^$\n]+?)\$/g;
  let cursor = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(text)) !== null) {
    if (match.index > cursor) content.push(textNode(text.slice(cursor, match.index)));
    const latex = match[1].trim();
    if (latex) {
      content.push({ type: "inlineMath", attrs: { latex } });
    } else {
      content.push(textNode(match[0]));
    }
    cursor = match.index + match[0].length;
  }

  if (cursor < text.length) content.push(textNode(text.slice(cursor)));
  return content;
}

function paragraphFromText(text: string): JSONContent | null {
  if (!text.trim()) return null;
  const content: JSONContent[] = [];
  const lines = text.split("\n");
  lines.forEach((line, index) => {
    if (index > 0) content.push({ type: "hardBreak" });
    content.push(...parseInlineText(line));
  });
  if (!content.some((node) => node.type !== "hardBreak")) return null;
  return { type: "paragraph", content };
}

function appendPlainText(documentContent: JSONContent[], text: string) {
  const normalized = text.replace(/\r\n?/g, "\n");
  const paragraphs = normalized.split(/\n[\t ]*\n/);
  paragraphs.forEach((paragraph) => {
    const node = paragraphFromText(paragraph);
    if (node) documentContent.push(node);
  });
}

export function ocrToEditorJSON(text: string): JSONContent {
  const content: JSONContent[] = [];
  const normalized = text.replace(/\r\n?/g, "\n");
  const blockPattern = /\$\$([\s\S]+?)\$\$/g;
  let cursor = 0;
  let match: RegExpExecArray | null;

  while ((match = blockPattern.exec(normalized)) !== null) {
    appendPlainText(content, normalized.slice(cursor, match.index));
    const latex = match[1].trim();
    if (latex) {
      content.push({ type: "blockMath", attrs: { latex } });
    } else {
      appendPlainText(content, match[0]);
    }
    cursor = match.index + match[0].length;
  }
  appendPlainText(content, normalized.slice(cursor));

  return { type: "doc", content: content.length > 0 ? content : [{ type: "paragraph" }] };
}
