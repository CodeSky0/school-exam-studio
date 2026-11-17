"use client";

import { RoleAccess } from "@/components/role-access";
import { AppShell } from "@/components/app-shell";
import { AgentChatWorkspace } from "./chat/agent-chat-workspace";

// AI 对话工作台对管理员与教师都可用，会话可见范围由后端按角色收敛。
export function AIHub() {
  return (
    <RoleAccess>
      {(user) => (
        <AppShell user={user}>
          <AgentChatWorkspace user={user} />
        </AppShell>
      )}
    </RoleAccess>
  );
}
