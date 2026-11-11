import type { Metadata } from "next";
import { TeacherPapers } from "@/components/paper/papers-workspace";

export const metadata: Metadata = { title: "组卷中心" };

export default function TeacherPapersPage() {
  return <TeacherPapers />;
}
