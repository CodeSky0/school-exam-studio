"use client";

import { RoleAccess } from "@/components/role-access";
import { AppShell } from "@/components/app-shell";
import { ProfileContent } from "@/components/profile/profile-content";

export function AccountPanel() {
  return <RoleAccess>{(user) => <AppShell user={user}><ProfileContent user={user} /></AppShell>}</RoleAccess>;
}
