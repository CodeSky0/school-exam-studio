"use client";

import { CheckCircledIcon, CrossCircledIcon, MinusCircledIcon, PlusCircledIcon } from "@radix-ui/react-icons";
import Link from "next/link";
import { aiAuditSchema, type AIRun, type AIRunStep } from "@/lib/ai";
import { cn } from "@/lib/utils";

export const intentLabels: Record<string, string> = {
  audit_bank: "题库体检",
  tag_questions: "分类与知识点整理",
  fill_explanations: "补答案与解析",
  organize_bank: "题库整理",
  generate_variants: "生成变式",
  generate_questions: "按知识点出题",
  compose_paper: "智能组卷",
  analyze_question: "分析题目",
  unknown: "未识别目标",
};

const statusStyles: Record<string, { label: string; className: string }> = {
  succeeded: { label: "已完成", className: "border-success/25 bg-success-soft text-success" },
  failed: { label: "失败", className: "border-error-soft-strong bg-error-soft text-error" },
  planned: { label: "计划中", className: "border-border bg-neutral-2 text-neutral-7" },
  skipped: { label: "已跳过", className: "border-warning/30 bg-warning-soft text-warning" },
};

export function formatAITime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false, timeZone: "Asia/Shanghai" }).format(date);
}

export function RunStatusBadge({ status }: { status: string }) {
  const style = statusStyles[status] ?? statusStyles.planned;
  return <span className={cn("inline-flex rounded-full border px-2 py-0.5 text-label-12 font-medium", style.className)}>{style.label}</span>;
}

function StepIcon({ status }: { status: string }) {
  if (status === "succeeded") return <CheckCircledIcon className="size-4 text-success" aria-hidden="true" />;
  if (status === "failed") return <CrossCircledIcon className="size-4 text-error" aria-hidden="true" />;
  if (status === "skipped") return <MinusCircledIcon className="size-4 text-warning" aria-hidden="true" />;
  return <PlusCircledIcon className="size-4 text-neutral-6" aria-hidden="true" />;
}

function StepList({ steps }: { steps: AIRunStep[] }) {
  if (steps.length === 0) return <p className="text-copy-13 text-neutral-7">没有步骤记录。</p>;
  return (
    <ol className="grid gap-3">
      {steps.map((step, index) => (
        <li key={`${step.key}-${index}`} className="grid grid-cols-[1.25rem_minmax(0,1fr)] gap-3">
          <div className="flex flex-col items-center">
            <StepIcon status={step.status} />
            {index < steps.length - 1 ? <span className="mt-1 w-px flex-1 bg-border" aria-hidden="true" /> : null}
          </div>
          <div className="pb-3">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-copy-13 font-medium text-neutral-9">{step.title}</p>
              {step.affected > 0 ? <span className="rounded bg-neutral-3 px-1.5 py-0.5 text-label-12 text-neutral-7">影响 {step.affected} 项</span> : null}
            </div>
            {step.summary ? <p className="mt-1 text-copy-13 leading-6 text-neutral-7">{step.summary}</p> : null}
            {step.warning ? <p className="mt-1 text-label-12 leading-5 text-warning">{step.warning}</p> : null}
            {step.question_ids?.length ? (
              <p className="mt-2 break-all font-mono text-label-12 leading-5 text-neutral-6">
                {step.question_ids.slice(0, 6).join(" · ")}
                {step.question_ids.length > 6 ? ` 等 ${step.question_ids.length} 道` : ""}
              </p>
            ) : null}
          </div>
        </li>
      ))}
    </ol>
  );
}

function AuditSummary({ audit }: { audit: ReturnType<typeof aiAuditSchema.parse> }) {
  const stats = [
    { label: "题目总数", value: audit.total },
    { label: "缺答案", value: audit.missing_answer },
    { label: "缺解析", value: audit.missing_explanation },
    { label: "缺知识点", value: audit.missing_knowledge_points },
    { label: "缺分类", value: audit.missing_category },
    { label: "重复组", value: audit.duplicate_count },
  ];
  return (
    <div className="rounded-lg border border-border bg-neutral-2/60 p-4">
      <h3 className="text-copy-13 font-medium text-neutral-9">题库体检报告</h3>
      <dl className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
        {stats.map((item) => (
          <div key={item.label} className="rounded-md border border-border bg-neutral-1 px-3 py-2">
            <dt className="text-label-12 text-neutral-6">{item.label}</dt>
            <dd className="mt-1 font-mono text-title-20 text-neutral-9">{item.value}</dd>
          </div>
        ))}
      </dl>
      {audit.findings.length ? (
        <ul className="mt-4 grid gap-2">
          {audit.findings.map((finding) => (
            <li key={finding.code} className="text-copy-13 leading-6 text-neutral-8">
              <span className={cn("mr-2 rounded px-1.5 py-0.5 text-label-12", finding.severity === "high" ? "bg-error-soft text-error" : finding.severity === "medium" ? "bg-warning-soft text-warning" : "bg-neutral-3 text-neutral-7")}>
                {finding.severity === "high" ? "高" : finding.severity === "medium" ? "中" : "低"}
              </span>
              {finding.message}
            </li>
          ))}
        </ul>
      ) : null}
      {audit.duplicates.length ? (
        <div className="mt-4">
          <h4 className="text-label-12 font-medium text-neutral-7">重复题预览</h4>
          <ul className="mt-2 grid gap-1.5">
            {audit.duplicates.slice(0, 5).map((group) => (
              <li key={group.content_hash} className="rounded border border-border bg-neutral-1 px-3 py-2 text-label-12 leading-5 text-neutral-7">
                <span className="font-mono text-neutral-6">{group.question_ids.length} 道 · </span>
                {group.preview || "（无可读题干）"}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

export function AgentRunReport({ run }: { run: AIRun }) {
  const audit = aiAuditSchema.safeParse(run.result.audit);
  const questionIDs = Array.isArray(run.result.question_ids) ? run.result.question_ids.filter((value): value is string => typeof value === "string") : [];
  const paperID = typeof run.result.paper_id === "string" ? run.result.paper_id : "";
  const paperTitle = typeof run.result.paper_title === "string" ? run.result.paper_title : "";
  const warnings = Array.isArray(run.result.warnings) ? run.result.warnings.filter((value): value is string => typeof value === "string") : [];

  return (
    <section aria-labelledby="agent-run-title" className="rounded-lg border border-border bg-neutral-1 p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-border pb-4">
        <div className="min-w-0">
          <p className="text-caption-10 font-medium uppercase tracking-[0.18em] text-neutral-6">Agent 运行报告</p>
          <h2 id="agent-run-title" className="mt-2 text-title-20 font-medium text-neutral-10">{intentLabels[run.intent] ?? run.intent}</h2>
          <p className="mt-1 max-w-3xl text-copy-13 leading-6 text-neutral-7">{run.goal}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <RunStatusBadge status={run.status} />
          <span className="rounded-full border border-border bg-neutral-2 px-2.5 py-1 font-mono text-label-12 text-neutral-7">{run.provider || "local"}</span>
          <span className="text-label-12 text-neutral-6">{formatAITime(run.created_at)}</span>
        </div>
      </div>

      {run.error ? <p role="alert" className="mt-4 rounded-lg border border-error-soft-strong bg-error-soft px-4 py-3 text-copy-13 leading-6 text-error">{run.error}</p> : null}

      <div className="mt-5 grid gap-6 xl:grid-cols-[minmax(0,22rem)_minmax(0,1fr)]">
        <div>
          <h3 className="text-label-12 font-medium tracking-wide text-neutral-7">执行步骤</h3>
          <div className="mt-3"><StepList steps={run.steps} /></div>
        </div>
        <div className="grid content-start gap-4">
          {audit.success ? <AuditSummary audit={audit.data} /> : null}
          {paperID ? (
            <div className="rounded-lg border border-accent/25 bg-accent-soft px-4 py-3">
              <h3 className="text-copy-13 font-medium text-accent-strong">已生成试卷</h3>
              <p className="mt-1 text-copy-13 leading-6 text-neutral-8">{paperTitle || paperID}</p>
              <Link href={`/teacher/papers/${paperID}/print`} className="mt-2 inline-block text-label-12 font-medium text-accent-strong underline underline-offset-2">打开 A4 打印预览</Link>
            </div>
          ) : null}
          {questionIDs.length ? (
            <div>
              <h3 className="text-label-12 font-medium tracking-wide text-neutral-7">涉及题目</h3>
              <p className="mt-2 break-all font-mono text-label-12 leading-6 text-neutral-7">{questionIDs.join(" · ")}</p>
            </div>
          ) : null}
          {warnings.length ? (
            <div>
              <h3 className="text-label-12 font-medium tracking-wide text-warning">需要注意</h3>
              <ul className="mt-2 grid gap-1.5 text-copy-13 leading-6 text-neutral-7">
                {warnings.map((warning) => <li key={warning}>· {warning}</li>)}
              </ul>
            </div>
          ) : null}
          {!audit.success && !paperID && questionIDs.length === 0 && warnings.length === 0 ? (
            <p className="text-copy-13 leading-6 text-neutral-7">本次运行没有产生变更。可以调整目标描述或补充年级、知识点后重试。</p>
          ) : null}
        </div>
      </div>
    </section>
  );
}
