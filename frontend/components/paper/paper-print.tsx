"use client";

import { useQuery } from "@tanstack/react-query";
import { FileTextIcon } from "@radix-ui/react-icons";
import { getPaper, paperQueryKeys } from "@/lib/papers";
import { FormError } from "@/components/auth-form";
import { LoadingScreen } from "@/components/loading-screen";
import { Button } from "@/components/ui/button";
import { PaperAccess } from "./paper-access";
import { PaperPreview } from "./paper-preview";

function PrintContent({ paperId }: { paperId: string }) {
  const paperQuery = useQuery({
    queryKey: paperQueryKeys.detail(paperId),
    queryFn: () => getPaper(paperId),
  });

  if (paperQuery.isPending) return <div className="no-print"><LoadingScreen /></div>;
  if (paperQuery.error || !paperQuery.data) {
    return (
      <main className="no-print mx-auto max-w-lg px-6 py-16">
        <h1 className="mb-5 text-title-24 font-medium text-neutral-10">无法读取试卷</h1>
        <FormError error={paperQuery.error} />
        <Button variant="outline" className="mt-5" onClick={() => { void paperQuery.refetch(); }}>重新读取</Button>
      </main>
    );
  }

  return (
    <main className="paper-print-page">
      <div className="print-toolbar no-print">
        <Button onClick={() => window.print()}><FileTextIcon />打印试卷</Button>
      </div>
      <PaperPreview paper={paperQuery.data} />
    </main>
  );
}

export function TeacherPaperPrint({ paperId }: { paperId: string }) {
  return <PaperAccess standalone>{() => <PrintContent paperId={paperId} />}</PaperAccess>;
}
