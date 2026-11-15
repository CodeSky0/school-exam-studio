"use client";

import { RoleAccess } from "@/components/role-access";
import { AppShell } from "@/components/app-shell";
import { AIWorkspace } from "./ai-workspace";

export function AdminAIPageContent() {
  return (
    <RoleAccess role="admin">
      {(user) => <AppShell user={user}><AIWorkspace user={user} scope="admin" /></AppShell>}
    </RoleAccess>
  );
}
