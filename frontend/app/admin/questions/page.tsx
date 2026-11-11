import type { Metadata } from "next";
import { AdminQuestionBank } from "@/components/admin/admin-question-bank";

export const metadata: Metadata = { title: "全校题库" };

export default function AdminQuestionsPage() {
  return <AdminQuestionBank />;
}
