import type { Metadata } from "next";
import { TeacherAIPageContent } from "@/components/ai/teacher-ai-page";

export const metadata: Metadata = { title: "AI 工作台" };

export default function TeacherAIPage() {
  return <TeacherAIPageContent />;
}
