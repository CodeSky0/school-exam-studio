"use client";

import { AppShell } from "@/components/app-shell";
import { RoleAccess } from "@/components/role-access";
import { ProfileContent } from "./profile-content";

export function TeacherProfile() {
  return <RoleAccess role="teacher">{(user) => <AppShell user={user}><ProfileContent user={user} /></AppShell>}</RoleAccess>;
}
