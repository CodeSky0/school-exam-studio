import type { Metadata } from "next";
import { AIHub } from "@/components/ai/ai-hub";

export const metadata: Metadata = { title: "AI 助手" };

export default function AIPage() {
  return <AIHub />;
}
