"use client";

import { useQuery } from "@tanstack/react-query";
import { LightningBoltIcon, LockClosedIcon, MagicWandIcon } from "@radix-ui/react-icons";
import { aiQueryKeys, getAIStatus } from "@/lib/ai";
import { cn } from "@/lib/utils";

const providerCopy: Record<string, { label: string; description: string; tone: string }> = {
  llm: {
    label: "大模型已配置",
    description: "分析、出题与补解析优先调用外部大模型，失败时自动降级为本地引擎。可在学校设置中点击“测试已保存配置”验证连通性。",
    tone: "border-accent/30 bg-accent-soft text-accent-strong",
  },
  local: {
    label: "本地确定性引擎",
    description: "当前未启用全校共享模型或服务器默认模型，使用内置规则完成分类、标签、出题与题库体检；结果均可复现。",
    tone: "border-info/25 bg-info-soft text-info",
  },
  local_fallback: {
    label: "本地降级模式",
    description: "大模型调用失败时自动使用本地引擎，并在结果中提示需要人工复核。",
    tone: "border-warning/30 bg-warning-soft text-warning",
  },
  none: {
    label: "AI 未启用",
    description: "当前没有可用的 AI 引擎。",
    tone: "border-border bg-neutral-2 text-neutral-7",
  },
};

export function AIStatusCard() {
  const statusQuery = useQuery({ queryKey: aiQueryKeys.status, queryFn: getAIStatus });
  const status = statusQuery.data;
  const provider = providerCopy[status?.provider ?? "local"] ?? providerCopy.local;

  return (
    <section aria-labelledby="ai-status-title" className="rounded-lg border border-border bg-neutral-1 p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <p className="text-caption-10 font-medium uppercase tracking-[0.18em] text-neutral-6">AI 引擎</p>
          <h2 id="ai-status-title" className="mt-2 flex items-center gap-2 text-title-20 font-medium text-neutral-10">
            <MagicWandIcon className="size-5 text-accent" />
            {statusQuery.isPending ? "正在读取引擎状态…" : provider.label}
          </h2>
          <p className="mt-2 max-w-2xl text-copy-13 leading-6 text-neutral-7">
            {statusQuery.isError ? "无法读取 AI 状态，请确认后端与登录会话可用。" : provider.description}
          </p>
          {status?.warning ? <p className="mt-2 text-label-12 leading-5 text-warning">{status.warning}</p> : null}
        </div>
        <div className="flex flex-wrap gap-2">
          <span className={cn("inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-label-12 font-medium", provider.tone)}>
            <LightningBoltIcon className="size-3.5" />
            {status?.provider ?? "local"}
          </span>
          {status?.source === "school" ? <span className="rounded-full border border-accent/30 bg-accent-soft px-2.5 py-1 text-label-12 text-accent-strong">学校共享配置</span> : null}
          {status?.source === "env" ? <span className="rounded-full border border-border bg-neutral-2 px-2.5 py-1 text-label-12 text-neutral-7">服务器默认配置</span> : null}
          {status?.model ? <span className="rounded-full border border-border bg-neutral-2 px-2.5 py-1 font-mono text-label-12 text-neutral-7">{status.model}</span> : null}
        </div>
      </div>

      {status ? (
        <div className="mt-5 grid gap-5 border-t border-border pt-5 lg:grid-cols-3">
          <div>
            <h3 className="text-label-12 font-medium tracking-wide text-neutral-7">能力</h3>
            <ul className="mt-2 grid gap-1.5 text-copy-13 leading-6 text-neutral-8">
              {status.capabilities.map((item) => <li key={item} className="flex gap-2"><span className="mt-2 size-1 shrink-0 rounded-full bg-accent" aria-hidden="true" />{item}</li>)}
            </ul>
          </div>
          <div>
            <h3 className="text-label-12 font-medium tracking-wide text-neutral-7">单次限制</h3>
            <dl className="mt-2 grid gap-1.5 font-mono text-label-12 text-neutral-8">
              <div className="flex justify-between gap-3"><dt>分析字符</dt><dd>{status.limits.max_analyze_chars}</dd></div>
              <div className="flex justify-between gap-3"><dt>生成题目</dt><dd>{status.limits.max_generate}</dd></div>
              <div className="flex justify-between gap-3"><dt>批量处理</dt><dd>{status.limits.max_batch}</dd></div>
            </dl>
          </div>
          <div>
            <h3 className="flex items-center gap-1.5 text-label-12 font-medium tracking-wide text-neutral-7"><LockClosedIcon className="size-3.5" />安全边界</h3>
            <ul className="mt-2 grid gap-1.5 text-label-12 leading-5 text-neutral-7">
              {status.safety.map((item) => <li key={item}>{item}</li>)}
            </ul>
          </div>
        </div>
      ) : null}
    </section>
  );
}
