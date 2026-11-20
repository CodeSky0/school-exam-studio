"use client";

import { AppShell } from "@/components/app-shell";
import { RoleAccess } from "@/components/role-access";
import { CloudDrive } from "./cloud-drive";

export function CloudWorkspace() {
  return (
    <RoleAccess>
      {(user) => (
        <AppShell user={user}>
          <CloudDrive />
        </AppShell>
      )}
    </RoleAccess>
  );
}
