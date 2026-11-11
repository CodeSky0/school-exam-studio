import type { Metadata } from "next";
import { AdminDashboard } from "@/components/admin/admin-dashboard";

export const metadata: Metadata = { title: "学校总览" };

export default function AdminDashboardPage() {
  return <AdminDashboard />;
}
