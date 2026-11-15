"use client";

import { RoleAccess } from "@/components/role-access";
import { AppShell } from "@/components/app-shell";
import { AIWorkspace } from "./ai-workspace";

// AI 工作台对管理员与教师都可用，具体可见范围由 AIWorkspace 按角色收敛。
export function AIHub() {
  return (
    <RoleAccess>
      {(user) => (
        <AppShell user={user}>
          <AIWorkspace user={user} scope={user.role === "admin" ? "admin" : "teacher"} />
        </AppShell>
      )}
    </RoleAccess>
  );
}
