"use client";

import { useEffect, useMemo, useRef } from "react";
import { sanitizeRichText } from "@/lib/rich-text";
import { cn } from "@/lib/utils";

export function RichTextView({ content, className }: { content: string; className?: string }) {
  const rootRef = useRef<HTMLDivElement>(null);
  const sanitized = useMemo(() => sanitizeRichText(content), [content]);

  useEffect(() => {
    const root = rootRef.current;
    if (!root) return;
    const mathElements = root.querySelectorAll<HTMLElement>('[data-type="inline-math"], [data-type="inlineMath"], [data-type="block-math"], [data-type="blockMath"]');
    if (mathElements.length === 0) return;
    let cancelled = false;
    // KaTeX 体积较大，只在页面确实包含公式时按需加载，避免拖慢首屏脚本解析。
    void import("katex")
      .then(({ default: katex }) => {
        if (cancelled) return;
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
      })
      .catch(() => {
        if (cancelled) return;
        mathElements.forEach((element) => {
          const latex = element.getAttribute("data-latex") ?? "";
          const displayMode = element.getAttribute("data-type") === "block-math" || element.getAttribute("data-type") === "blockMath";
          element.textContent = displayMode ? `$$${latex}$$` : `$${latex}$`;
        });
      });
    return () => {
      cancelled = true;
    };
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
