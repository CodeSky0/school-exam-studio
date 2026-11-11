import type { Metadata } from "next";
import { RegisterForm } from "@/components/auth/register-form";

export const metadata: Metadata = { title: "教师注册" };

export default function RegisterPage() {
  return <RegisterForm />;
}
