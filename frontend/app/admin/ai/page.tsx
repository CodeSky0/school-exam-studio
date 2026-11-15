import type { Metadata } from "next";
import { AdminAIPageContent } from "@/components/ai/admin-ai-page";

export const metadata: Metadata = { title: "AI 工作台" };

export default function AdminAIPage() {
  return <AdminAIPageContent />;
}
