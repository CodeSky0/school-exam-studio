"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronRightIcon, ClockIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { aiQueryKeys, listAgentRuns, listAIAuditLogs, type AIRun } from "@/lib/ai";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { AgentConsole } from "./agent-console";
import { AgentRunReport, RunStatusBadge, formatAITime, intentLabels } from "./agent-run-report";
import { AIStatusCard } from "./ai-status-card";

export function AIWorkspace({ user, scope }: { user: CurrentUser; scope: "teacher" | "admin" }) {
  const [activeRun, setActiveRun] = useState<AIRun | null>(null);
  const runsQuery = useQuery({ queryKey: aiQueryKeys.runs(20), queryFn: () => listAgentRuns(20) });
  const logsQuery = useQuery({
    queryKey: aiQueryKeys.logs(30),
    queryFn: () => listAIAuditLogs(30),
    enabled: scope === "admin",
  });

  return (
    <div className="mx-auto grid max-w-[1500px] gap-6">
      <PageHeader
        eyebrow={scope === "admin" ? "管理员工作区 / 智能化" : "教师工作区 / 智能化"}
        title="AI 工作台"
        description="让应用内的 AI Agent 代替重复的题库维护工作：体检、分类、补解析、出题、变式与组卷。"
        actions={<span className="rounded-full border border-border bg-neutral-1 px-3 py-1.5 text-label-12 text-neutral-7">{user.role === "teacher" ? "教师权限" : "管理员权限"}</span>}
      />

      <AIStatusCard />

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="grid content-start gap-6">
          <AgentConsole user={user} onCreated={setActiveRun} />
          {activeRun ? <AgentRunReport run={activeRun} /> : (
            <section className="rounded-lg border border-dashed border-border-strong bg-neutral-1/60 px-5 py-10 text-center">
              <h2 className="text-copy-13 font-medium text-neutral-9">还没有运行结果</h2>
              <p className="mt-2 text-copy-13 leading-6 text-neutral-7">在上方描述目标或选择常用指令，Agent 会展示计划、步骤和实际影响。</p>
            </section>
          )}
        </div>

        <aside className="grid content-start gap-6">
          <section aria-labelledby="recent-runs-title" className="rounded-lg border border-border bg-neutral-1 p-4 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <h2 id="recent-runs-title" className="text-copy-13 font-medium text-neutral-9">最近运行</h2>
              <ClockIcon className="size-4 text-neutral-6" />
            </div>
            {runsQuery.isPending ? <p className="mt-3 text-label-12 text-neutral-7">正在读取…</p> : null}
            {runsQuery.isError ? <p className="mt-3 text-label-12 leading-5 text-error">无法读取运行记录。</p> : null}
            {runsQuery.data && runsQuery.data.items.length === 0 ? <p className="mt-3 text-label-12 leading-5 text-neutral-7">暂无记录。运行一次 Agent 后会显示在这里。</p> : null}
            <ul className="mt-3 grid gap-2">
              {runsQuery.data?.items.map((run) => (
                <li key={run.id}>
                  <button
                    type="button"
                    onClick={() => setActiveRun(run)}
                    className="w-full rounded-md border border-border bg-neutral-1 px-3 py-2.5 text-left transition-colors hover:border-accent/40 hover:bg-accent-soft/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/35"
                  >
                    <span className="flex items-center justify-between gap-2">
                      <span className="text-label-12 font-medium text-neutral-9">{intentLabels[run.intent] ?? run.intent}</span>
                      <RunStatusBadge status={run.status} />
                    </span>
                    <span className="mt-1 block truncate text-label-12 text-neutral-7">{run.goal}</span>
                    <span className="mt-1 block text-caption-10 uppercase tracking-[0.12em] text-neutral-6">{formatAITime(run.created_at)}</span>
                  </button>
                </li>
              ))}
            </ul>
          </section>

          {scope === "admin" ? (
            <section aria-labelledby="ai-audit-title" className="rounded-lg border border-border bg-neutral-1 p-4 shadow-sm">
              <h2 id="ai-audit-title" className="text-copy-13 font-medium text-neutral-9">AI 审计日志</h2>
              <p className="mt-1 text-label-12 leading-5 text-neutral-7">最近 30 条 AI 写入记录，用于追溯自动化操作。</p>
              {logsQuery.isPending ? <p className="mt-3 text-label-12 text-neutral-7">正在读取…</p> : null}
              {logsQuery.isError ? <p className="mt-3 text-label-12 leading-5 text-neutral-7">暂无权限或读取失败。</p> : null}
              {logsQuery.data && logsQuery.data.items.length === 0 ? <p className="mt-3 text-label-12 text-neutral-7">暂无审计记录。</p> : null}
              <ul className="mt-3 grid gap-2">
                {logsQuery.data?.items.map((log) => (
                  <li key={log.id} className="rounded-md border border-border px-3 py-2">
                    <p className="text-label-12 font-medium text-neutral-8">{log.action}</p>
                    <p className="mt-1 break-all font-mono text-caption-10 text-neutral-6">{log.target_type} · {log.target_id ?? "—"}</p>
                    <p className="mt-1 text-caption-10 text-neutral-6">{formatAITime(log.created_at)}</p>
                  </li>
                ))}
              </ul>
              {logsQuery.data && logsQuery.data.items.length > 0 ? (
                <Button variant="ghost" size="sm" className="mt-3 w-full" onClick={() => { void logsQuery.refetch(); }}>刷新日志<ChevronRightIcon /></Button>
              ) : null}
            </section>
          ) : null}
        </aside>
      </div>
    </div>
  );
}
