"use client";

import { useRouter } from "next/navigation";
import { AppShell } from "@/components/app-shell";
import { PaperAccess } from "./paper-access";
import { PaperStoreProvider } from "./paper-store-provider";
import { PaperComposer } from "./paper-composer";
import { PaperList } from "./paper-list";

export function TeacherPapers() {
  const router = useRouter();
  return (
    <PaperAccess>
      {(user) => (
        <AppShell user={user}>
          <PaperStoreProvider userId={user.id}>
            <PaperComposer user={user} onSaved={(paper) => router.push(`/teacher/papers/${paper.id}`)} />
          </PaperStoreProvider>
          <div className="mx-auto max-w-[1500px]"><PaperList /></div>
        </AppShell>
      )}
    </PaperAccess>
  );
}

export function TeacherPaperEditor({ paperId }: { paperId: string }) {
  return (
    <PaperAccess>
      {(user) => (
        <AppShell user={user}>
          <PaperStoreProvider userId={user.id}>
            <PaperComposer user={user} paperId={paperId} />
          </PaperStoreProvider>
        </AppShell>
      )}
    </PaperAccess>
  );
}
