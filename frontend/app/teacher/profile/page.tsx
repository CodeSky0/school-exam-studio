import type { Metadata } from "next";
import { TeacherProfile } from "@/components/profile/teacher-profile";

export const metadata: Metadata = { title: "个人中心" };

export default function TeacherProfilePage() {
  return <TeacherProfile />;
}
