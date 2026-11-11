import type { Metadata } from "next";
import { TeacherPaperPrint } from "@/components/paper/paper-print";

export const metadata: Metadata = { title: "打印试卷" };

export default async function TeacherPaperPrintPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <TeacherPaperPrint paperId={id} />;
}
