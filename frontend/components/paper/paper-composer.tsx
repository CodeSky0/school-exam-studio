"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeftIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { getPaper, paperQueryKeys } from "@/lib/papers";
import type { Paper } from "@/lib/paper-schemas";
import { FormError } from "@/components/auth-form";
import { Button, buttonVariants } from "@/components/ui/button";
import { usePaperStore } from "./paper-store-provider";
import { QuestionPicker } from "./question-picker";
import { PaperCart } from "./paper-cart";
import { PaperForm } from "./paper-form";
import { LivePaperPreview } from "./paper-preview";

export function PaperComposer({ user, paperId, onSaved }: {
  user: CurrentUser;
  paperId?: string;
  onSaved?: (paper: Paper) => void;
}) {
  const [savedMessage, setSavedMessage] = useState("");
  const currentPaperId = usePaperStore((state) => state.paperId);
  const ensureNewPaper = usePaperStore((state) => state.ensureNewPaper);
  const loadPaper = usePaperStore((state) => state.loadPaper);
  const paperQuery = useQuery({
    queryKey: paperQueryKeys.detail(paperId ?? "new"),
    queryFn: () => getPaper(paperId as string),
    enabled: Boolean(paperId),
  });

  useEffect(() => {
    if (!paperId) {
      ensureNewPaper(user.subject);
      return;
    }
    if (paperQuery.data && currentPaperId !== paperId) loadPaper(paperQuery.data);
  }, [currentPaperId, ensureNewPaper, loadPaper, paperId, paperQuery.data, user.subject]);

  if (paperId && paperQuery.isPending) {
    return (
      <div className="mx-auto max-w-[1500px] animate-pulse" aria-label="正在读取试卷">
        <div className="h-4 w-32 rounded bg-slate-200" /><div className="mt-4 h-9 w-72 max-w-full rounded bg-slate-200" />
        <div className="mt-8 grid gap-6 xl:grid-cols-2"><div className="h-96 rounded-xl bg-slate-100" /><div className="h-96 rounded-xl bg-slate-100" /></div>
      </div>
    );
  }

  if (paperId && paperQuery.error) {
    return (
      <div className="mx-auto max-w-2xl py-10">
        <Link href="/teacher/papers" className={buttonVariants({ variant: "ghost", size: "sm", className: "mb-5 -ml-3" })}><ArrowLeftIcon />返回组卷中心</Link>
        <FormError error={paperQuery.error} />
        <Button variant="outline" size="sm" className="mt-4" onClick={() => { void paperQuery.refetch(); }}>重新读取</Button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-[1500px]">
      <header className="pb-6">
        {paperId ? <Link href="/teacher/papers" className={buttonVariants({ variant: "ghost", size: "sm", className: "mb-3 -ml-3" })}><ArrowLeftIcon />返回组卷中心</Link> : null}
        <p className="text-xs font-semibold tracking-[0.16em] text-primary">教师工作区 / 组卷中心</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">{paperId ? "编辑与预览试卷" : "组卷中心"}</h1>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">从本校已发布题库选题，安排卷面顺序和分值。未保存内容仅保留在当前教师的浏览器中。</p>
      </header>
      {savedMessage ? <p role="status" className="mb-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">{savedMessage}</p> : null}
      <div className="grid items-start gap-6 2xl:grid-cols-[minmax(0,1.4fr)_minmax(22rem,0.8fr)]">
        <QuestionPicker defaultSubject={user.subject} />
        <aside className="grid min-w-0 gap-5" aria-label="试卷编辑面板">
          <PaperCart />
          <PaperForm paperId={paperId} onSaved={(paper) => {
            setSavedMessage("试卷修改已保存");
            onSaved?.(paper);
          }} />
        </aside>
      </div>
      {paperQuery.data ? (
        <section className="mt-10 min-w-0" aria-labelledby="paper-preview-heading">
          <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
            <div>
              <h2 id="paper-preview-heading" className="text-xl font-semibold tracking-tight text-slate-950">A4 卷面预览</h2>
              <p className="mt-1 text-xs leading-5 text-slate-500">预览当前编辑内容，不显示答案和解析；打印页使用最后保存的服务端版本。</p>
            </div>
          </div>
          <div className="paper-preview-stage"><LivePaperPreview paper={paperQuery.data} /></div>
        </section>
      ) : null}
    </div>
  );
}
