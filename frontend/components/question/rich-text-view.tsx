"use client";

import { useEffect, useMemo, useRef } from "react";
import katex from "katex";
import { sanitizeRichText } from "@/lib/rich-text";
import { cn } from "@/lib/utils";

export function RichTextView({ content, className }: { content: string; className?: string }) {
  const rootRef = useRef<HTMLDivElement>(null);
  const sanitized = useMemo(() => sanitizeRichText(content), [content]);

  useEffect(() => {
    const root = rootRef.current;
    if (!root) return;
    const mathElements = root.querySelectorAll<HTMLElement>('[data-type="inline-math"], [data-type="inlineMath"], [data-type="block-math"], [data-type="blockMath"]');
    mathElements.forEach((element) => {
      const latex = element.getAttribute("data-latex") ?? "";
      const dataType = element.getAttribute("data-type");
      const displayMode = dataType === "block-math" || dataType === "blockMath";
      element.replaceChildren();
      try {
        katex.render(latex, element, { displayMode, throwOnError: false, strict: "error", trust: false, output: "html" });
      } catch {
        element.textContent = displayMode ? `$$${latex}$$` : `$${latex}$`;
      }
    });
  }, [sanitized]);

  if (!content) return <span className="text-neutral-6">未填写</span>;
  return (
    <div
      ref={rootRef}
      className={cn("question-rich-text", className)}
      dangerouslySetInnerHTML={{ __html: sanitized }}
    />
  );
}
