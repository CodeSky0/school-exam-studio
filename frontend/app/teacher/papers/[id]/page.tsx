import type { Metadata } from "next";
import { TeacherPaperEditor } from "@/components/paper/papers-workspace";

export const metadata: Metadata = { title: "编辑试卷" };

export default async function TeacherPaperEditorPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <TeacherPaperEditor paperId={id} />;
}
