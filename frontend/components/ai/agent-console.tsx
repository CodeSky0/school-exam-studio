"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { PaperPlaneIcon, ResetIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { aiQueryKeys, createAgentRun, type AIRun } from "@/lib/ai";
import { FormError } from "@/components/auth-form";
import { questionQueryKeys } from "@/lib/questions";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

const presets = [
  { label: "题库体检", goal: "对当前题库做一次质量体检，找出重复题和缺少解析的题目" },
  { label: "整理分类", goal: "整理我的草稿，补齐分类、知识点和标签" },
  { label: "补解析", goal: "为缺少答案或解析的草稿补充答案和解析" },
  { label: "出三题", goal: "八年级数学按知识点「一元一次方程」生成 3 道新题" },
  { label: "做变式", goal: "选一道我的草稿生成 3 道变式题" },
  { label: "智能组卷", goal: "用已发布题目为八年级数学组一份 100 分的试卷" },
];

interface AgentConsoleProps {
  user: CurrentUser;
  onCreated: (run: AIRun) => void;
}

export function AgentConsole({ user, onCreated }: AgentConsoleProps) {
  const queryClient = useQueryClient();
  const [goal, setGoal] = useState("");
  const [grade, setGrade] = useState("");
  const [subject, setSubject] = useState(user.subject || "数学");
  const [category, setCategory] = useState("");
  const [knowledgePoint, setKnowledgePoint] = useState("");
  const [tag, setTag] = useState("");
  const [questionID, setQuestionID] = useState("");
  const [count, setCount] = useState("");

  const mutation = useMutation({
    mutationFn: () => createAgentRun({
      goal: goal.trim(),
      ...(subject.trim() ? { subject: subject.trim() } : {}),
      ...(grade.trim() ? { grade: grade.trim() } : {}),
      ...(category.trim() ? { category: category.trim() } : {}),
      ...(knowledgePoint.trim() ? { knowledge_point: knowledgePoint.trim() } : {}),
      ...(tag.trim() ? { tag: tag.trim() } : {}),
      ...(questionID.trim() ? { question_id: questionID.trim() } : {}),
      ...(Number(count) > 0 ? { count: Number(count) } : {}),
    }),
    onSuccess: async (run) => {
      onCreated(run);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: aiQueryKeys.runs(20) }),
        queryClient.invalidateQueries({ queryKey: questionQueryKeys.all }),
      ]);
    },
  });

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!goal.trim()) return;
    mutation.mutate();
  }

  return (
    <section aria-labelledby="agent-console-title" className="rounded-lg border border-border bg-neutral-1 p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-caption-10 font-medium uppercase tracking-[0.18em] text-neutral-6">AI Agent</p>
          <h2 id="agent-console-title" className="mt-2 text-title-20 font-medium text-neutral-10">用一句话交给 AI 执行</h2>
          <p className="mt-2 max-w-2xl text-copy-13 leading-6 text-neutral-7">
            Agent 会先识别意图并生成计划，再调用题库工具执行。所有写入都受草稿权限约束，并记录审计日志。
          </p>
        </div>
        <span className="rounded-full border border-border bg-neutral-2 px-2.5 py-1 text-label-12 text-neutral-7">
          {user.role === "teacher" ? `学科固定为 ${user.subject}` : "管理员可跨教师整理本校草稿"}
        </span>
      </div>

      <form onSubmit={submit} className="mt-5 grid gap-4">
        <div className="grid gap-2">
          <Label htmlFor="agent-goal">目标描述</Label>
          <Textarea
            id="agent-goal"
            value={goal}
            maxLength={500}
            placeholder="例如：把八年级数学草稿里缺少分类和知识点的题目整理一下"
            aria-describedby="agent-goal-hint"
            onChange={(event) => setGoal(event.target.value)}
          />
          <p id="agent-goal-hint" className="text-label-12 leading-5 text-neutral-7">支持：题库体检、批量整理分类/知识点、补答案与解析、生成变式、按知识点出题、智能组卷。</p>
        </div>

        <div className="flex flex-wrap gap-2" aria-label="常用指令">
          {presets.map((preset) => (
            <Button key={preset.label} type="button" variant="secondary" size="sm" onClick={() => setGoal(preset.goal)}>{preset.label}</Button>
          ))}
        </div>

        <details className="rounded-lg border border-border bg-neutral-2/50 px-4 py-3">
          <summary className="cursor-pointer text-copy-13 font-medium text-neutral-8">可选参数</summary>
          <div className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {user.role === "admin" ? (
              <div className="grid gap-1.5">
                <Label htmlFor="agent-subject" className="text-label-12">学科</Label>
                <Input id="agent-subject" className="h-9" value={subject} placeholder="例如：数学" onChange={(event) => setSubject(event.target.value)} />
              </div>
            ) : null}
            <div className="grid gap-1.5">
              <Label htmlFor="agent-grade" className="text-label-12">年级</Label>
              <Input id="agent-grade" className="h-9" value={grade} placeholder="例如：八年级" onChange={(event) => setGrade(event.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="agent-category" className="text-label-12">分类</Label>
              <Input id="agent-category" className="h-9" value={category} placeholder="例如：方程与不等式" onChange={(event) => setCategory(event.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="agent-knowledge" className="text-label-12">知识点</Label>
              <Input id="agent-knowledge" className="h-9" value={knowledgePoint} placeholder="例如：一元一次方程" onChange={(event) => setKnowledgePoint(event.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="agent-tag" className="text-label-12">标签</Label>
              <Input id="agent-tag" className="h-9" value={tag} placeholder="例如：计算题" onChange={(event) => setTag(event.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="agent-question" className="text-label-12">题目 ID</Label>
              <Input id="agent-question" className="h-9" value={questionID} placeholder="分析或变式的原题 ID" onChange={(event) => setQuestionID(event.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="agent-count" className="text-label-12">数量</Label>
              <Input id="agent-count" className="h-9" type="number" min={1} max={10} value={count} placeholder="出题 / 变式 / 组卷题数" onChange={(event) => setCount(event.target.value)} />
            </div>
          </div>
        </details>

        <FormError error={mutation.error} />

        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-label-12 leading-5 text-neutral-7">AI 生成内容一律标记为待人工复核；已发布题目只读。</p>
          <div className="flex gap-2">
            <Button type="button" variant="ghost" size="sm" disabled={mutation.isPending} onClick={() => { mutation.reset(); setGoal(""); }}><ResetIcon />清空</Button>
            <Button type="submit" disabled={mutation.isPending || !goal.trim()}>
              <PaperPlaneIcon />
              {mutation.isPending ? "AI 正在执行…" : "让 AI 执行计划"}
            </Button>
          </div>
        </div>
      </form>
    </section>
  );
}
