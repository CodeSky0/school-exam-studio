import type { Metadata } from "next";
import { SchoolSettings } from "@/components/admin/school-settings";

export const metadata: Metadata = { title: "学校设置" };

export default function AdminSettingsPage() {
  return <SchoolSettings />;
}
