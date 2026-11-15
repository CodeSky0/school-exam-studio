"use client";

import { RoleAccess } from "@/components/role-access";
import { AppShell } from "@/components/app-shell";
import { AIWorkspace } from "./ai-workspace";

export function TeacherAIPageContent() {
  return (
    <RoleAccess role="teacher">
      {(user) => <AppShell user={user}><AIWorkspace user={user} scope="teacher" /></AppShell>}
    </RoleAccess>
  );
}
