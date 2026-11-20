import type { Metadata } from "next";
import { CloudWorkspace } from "@/components/cloud/cloud-workspace";

export const metadata: Metadata = { title: "云盘" };

export default function CloudPage() {
  return <CloudWorkspace />;
}
