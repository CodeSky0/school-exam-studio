"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeftIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { getPaper, paperQueryKeys } from "@/lib/papers";
import type { Paper } from "@/lib/paper-schemas";
import { FormError } from "@/components/auth-form";
import { PageHeader } from "@/components/page-header";
import { Button, buttonVariants } from "@/components/ui/button";
import { StatusMessage } from "@/components/ui/status-message";
import { PaperComposerSkeleton, usePaperStore } from "./paper-store-provider";
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
    return <PaperComposerSkeleton label="正在读取试卷" />;
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
      <PageHeader
        eyebrow="教师工作区 / 组卷中心"
        title={paperId ? "编辑与预览试卷" : "组卷中心"}
        description="从本校已发布题库选题，安排卷面顺序和分值。未保存内容仅保留在当前教师的浏览器中。"
        back={paperId ? <Link href="/teacher/papers" className={buttonVariants({ variant: "ghost", size: "sm", className: "-ml-3" })}><ArrowLeftIcon />返回组卷中心</Link> : undefined}
      />
      <StatusMessage message={savedMessage} tone="success" className="mb-5" />
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
              <h2 id="paper-preview-heading" className="text-title-20 font-medium tracking-[0.01em] text-neutral-10">A4 卷面预览</h2>
              <p className="mt-1 text-label-12 leading-5 text-neutral-7">预览当前编辑内容，不显示答案和解析；打印页使用最后保存的服务端版本。</p>
            </div>
          </div>
          <div className="paper-preview-stage"><LivePaperPreview paper={paperQuery.data} /></div>
        </section>
      ) : null}
    </div>
  );
}
