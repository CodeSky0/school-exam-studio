import type { Metadata } from "next";
import { AccountPanel } from "@/components/account-panel";

export const metadata: Metadata = { title: "个人资料" };

export default function AccountPage() {
  return <AccountPanel />;
}
