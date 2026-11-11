import type { Metadata } from "next";
import { TeacherDashboard } from "@/components/profile/teacher-dashboard";

export const metadata: Metadata = { title: "个人总览" };

export default function TeacherDashboardPage() {
  return <TeacherDashboard />;
}
