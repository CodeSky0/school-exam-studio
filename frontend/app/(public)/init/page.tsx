import type { Metadata } from "next";
import { InitPanel } from "@/components/auth/init-panel";

export const metadata: Metadata = { title: "初始化学校" };

export default function InitPage() {
  return <InitPanel />;
}
