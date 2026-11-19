"use client";

import { useEffect, useRef } from "react";
import { cn } from "@/lib/utils";

interface MessageMathProps {
  latex: string;
  display?: boolean;
  className?: string;
}

// MessageMath 渲染 AI 对话中的 LaTeX 公式，KaTeX 按需加载，避免增加首屏脚本体积。
export function MessageMath({ latex, display = false, className }: MessageMathProps) {
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    let cancelled = false;
    element.textContent = display ? `$$${latex}$$` : `$${latex}$`;
    void import("katex")
      .then(({ default: katex }) => {
        if (cancelled || !ref.current) return;
        try {
          katex.render(latex, ref.current, {
            displayMode: display,
            throwOnError: false,
            strict: "ignore",
            trust: false,
            output: "html",
          });
        } catch {
          ref.current.textContent = display ? `$$${latex}$$` : `$${latex}$`;
        }
      })
      .catch(() => {
        if (cancelled || !ref.current) return;
        ref.current.textContent = display ? `$$${latex}$$` : `$${latex}$`;
      });
    return () => {
      cancelled = true;
    };
  }, [latex, display]);

  return <span ref={ref} className={cn(display ? "my-1 block overflow-x-auto text-center" : "inline align-baseline", className)} />;
}
