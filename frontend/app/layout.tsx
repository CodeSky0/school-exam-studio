import type { Metadata } from "next";
import { Providers } from "@/components/providers";
import "katex/dist/katex.min.css";
import "./globals.css";
import "./print.css";

export const metadata: Metadata = {
  title: { default: "校本题库", template: "%s | 校本题库" },
  description: "面向学校管理员与学科教师的题库与组卷系统",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="zh-CN">
      <body><Providers>{children}</Providers></body>
    </html>
  );
}
