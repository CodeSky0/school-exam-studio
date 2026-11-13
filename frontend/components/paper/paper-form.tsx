"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FileTextIcon } from "@radix-ui/react-icons";
import { createPaper, paperQueryKeys, updatePaper } from "@/lib/papers";
import { paperInputSchema, type Paper, type PaperInput } from "@/lib/paper-schemas";
import { FormError } from "@/components/auth-form";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { usePaperStore } from "./paper-store-provider";

export function PaperForm({ paperId, onSaved }: { paperId?: string; onSaved: (paper: Paper) => void }) {
  const queryClient = useQueryClient();
  const title = usePaperStore((state) => state.title);
  const subject = usePaperStore((state) => state.subject);
  const grade = usePaperStore((state) => state.grade);
  const durationMinutes = usePaperStore((state) => state.durationMinutes);
  const items = usePaperStore((state) => state.items);
  const setMetadata = usePaperStore((state) => state.setMetadata);
  const loadPaper = usePaperStore((state) => state.loadPaper);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const saveMutation = useMutation({
    mutationFn: (input: PaperInput) => paperId ? updatePaper(paperId, input) : createPaper(input),
    onSuccess: async (paper) => {
      loadPaper(paper);
      await queryClient.invalidateQueries({ queryKey: paperQueryKeys.all });
      onSaved(paper);
    },
  });

  function savePaper(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const result = paperInputSchema.safeParse({
      title,
      subject,
      grade,
      duration_minutes: /^\d+$/.test(durationMinutes) ? Number(durationMinutes) : Number.NaN,
      questions: items.map((item) => ({ question_id: item.question.id, assigned_score: item.assignedScore })),
    });
    if (!result.success) {
      const nextErrors: Record<string, string> = {};
      result.error.issues.forEach((issue) => {
        const path = issue.path.join(".") || "form";
        const key = issue.path[0] === "questions" && issue.path.length > 1 ? "questions" : path;
        if (!nextErrors[key]) nextErrors[key] = key === "questions" && issue.path.length > 1 ? "请检查每题分值，须为大于 0 的两位小数" : issue.message;
      });
      setErrors(nextErrors);
      return;
    }
    setErrors({});
    saveMutation.mutate(result.data);
  }

  return (
    <form onSubmit={savePaper} noValidate className="rounded-lg border border-border bg-neutral-1 p-5 shadow-sm" aria-labelledby="paper-form-title">
      <div className="border-b border-border pb-4">
        <h2 id="paper-form-title" className="text-title-20 font-medium text-neutral-10">卷面配置</h2>
        <p className="mt-1 text-label-12 leading-5 text-neutral-7">保存后可在详情页继续编辑、预览和打印。</p>
      </div>
      <div className="mt-4 grid gap-4">
        <div className="grid gap-2">
          <Label htmlFor="paper-title">试卷标题</Label>
          <Input id="paper-title" value={title} maxLength={200} placeholder="例如：八年级数学阶段练习" aria-invalid={Boolean(errors.title)} aria-describedby={errors.title ? "paper-title-error" : undefined} onChange={(event) => setMetadata({ title: event.target.value })} />
          {errors.title ? <p id="paper-title-error" className="text-label-12 text-error">{errors.title}</p> : null}
        </div>
        <div className="grid gap-4 sm:grid-cols-2 2xl:grid-cols-1">
          <div className="grid gap-2">
            <Label htmlFor="paper-subject">学科</Label>
            <Input id="paper-subject" value={subject} maxLength={30} aria-invalid={Boolean(errors.subject)} aria-describedby={errors.subject ? "paper-subject-error" : undefined} onChange={(event) => setMetadata({ subject: event.target.value })} />
            {errors.subject ? <p id="paper-subject-error" className="text-label-12 text-error">{errors.subject}</p> : null}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="paper-grade">年级</Label>
            <Input id="paper-grade" value={grade} maxLength={30} placeholder="例如：八年级" aria-invalid={Boolean(errors.grade)} aria-describedby={errors.grade ? "paper-grade-error" : undefined} onChange={(event) => setMetadata({ grade: event.target.value })} />
            {errors.grade ? <p id="paper-grade-error" className="text-label-12 text-error">{errors.grade}</p> : null}
          </div>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="paper-duration">考试时长（分钟）</Label>
          <Input id="paper-duration" type="number" min={1} max={600} step={1} inputMode="numeric" value={durationMinutes} aria-invalid={Boolean(errors.duration_minutes)} aria-describedby="paper-duration-message" onChange={(event) => setMetadata({ durationMinutes: event.target.value })} />
          <p id="paper-duration-message" className={errors.duration_minutes ? "text-label-12 text-error" : "text-label-12 text-neutral-7"}>{errors.duration_minutes ?? "可设置 1 至 600 分钟"}</p>
        </div>
      </div>
      {errors.questions ? <p role="alert" className="mt-4 rounded-lg border border-error-soft-strong bg-error-soft px-3 py-2 text-label-12 leading-5 text-error">{errors.questions}</p> : null}
      {saveMutation.isError ? <div className="mt-4"><FormError error={saveMutation.error} /></div> : null}
      <div className="mt-5 flex flex-wrap gap-3 border-t border-border pt-4">
        <Button type="submit" className="flex-1" disabled={saveMutation.isPending}>{saveMutation.isPending ? "正在保存…" : paperId ? "保存修改" : "创建试卷"}</Button>
        {paperId ? <Link href={`/teacher/papers/${paperId}/print`} className={buttonVariants({ variant: "outline" })}><FileTextIcon />打印预览</Link> : null}
      </div>
    </form>
  );
}
