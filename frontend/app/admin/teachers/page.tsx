import type { Metadata } from "next";
import { TeacherManagement } from "@/components/admin/teacher-management";

export const metadata: Metadata = { title: "教师管理" };

export default function AdminTeachersPage() {
  return <TeacherManagement />;
}
