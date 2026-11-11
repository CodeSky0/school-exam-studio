import type { Metadata } from "next";
import { TeacherQuestions } from "@/components/question/teacher-questions";

export const metadata: Metadata = { title: "教师题库" };

export default function TeacherQuestionsPage() {
  return <TeacherQuestions />;
}
