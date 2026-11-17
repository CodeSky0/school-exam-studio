"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation } from "@tanstack/react-query";
import { CheckIcon, MagicWandIcon } from "@radix-ui/react-icons";
import { analyzeDraft, type AIAnalysis } from "@/lib/ai";
import { formatTags, isChoiceType, parseTags, type QuestionFormState } from "@/lib/question-form";
import { normalizeChoiceAnswer } from "@/lib/question-schemas";
import { richTextHasContent } from "@/lib/rich-text";
import { FormError } from "@/components/auth-form";
import { Button } from "@/components/ui/button";

function toRichText(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return "";
  // 只把常见富文本标签视为 HTML，避免 "x<y 且 y>0" 这类数学表达式被误判。
  if (/<\/?(p|br|strong|b|em|i|u|s|span|div|ul|ol|li|sup|sub|img|table|thead|tbody|tr|td|th|blockquote|code|pre|hr)\b[^>]*>/i.test(trimmed)) return trimmed;
  const escaped = trimmed.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return `<p>${escaped}</p>`;
}

function buildPatch(state: QuestionFormState, analysis: AIAnalysis): Partial<QuestionFormState> {
  const patch: Partial<QuestionFormState> = {};
  if (analysis.category) patch.category = analysis.category;
  if (analysis.knowledge_points.length > 0) patch.knowledgePointsText = formatTags(analysis.knowledge_points);
  const mergedTags = Array.from(new Set([...parseTags(state.tagsText), ...analysis.tags])).slice(0, 10);
  if (mergedTags.length > 0) patch.tagsText = formatTags(mergedTags);
  if (analysis.difficulty >= 1 && analysis.difficulty <= 5) patch.difficulty = analysis.difficulty;

  if (!richTextHasContent(state.answer) && analysis.answer.trim()) {
    if (isChoiceType(state.question_type)) {
      const normalized = normalizeChoiceAnswer(analysis.answer);
      const valid = state.options.map((option) => option.key);
      const keys = normalized.split("").filter((key) => valid.includes(key as typeof valid[number]));
      if (keys.length > 0) patch.answer = state.question_type === "single_choice" ? keys[0] : keys.join(",");
    } else if (state.question_type === "true_false") {
      const normalized = analysis.answer.trim().toLowerCase();
      if (["true", "正确", "对"].includes(normalized)) patch.answer = "true";
      if (["false", "错误", "错"].includes(normalized)) patch.answer = "false";
    } else {
      patch.answer = toRichText(analysis.answer);
    }
  }
  if (!richTextHasContent(state.explanation) && analysis.explanation.trim()) {
    patch.explanation = toRichText(analysis.explanation);
  }
  return patch;
}

export function QuestionAIAssistant({ state, questionID, onApply }: {
  state: QuestionFormState;
  questionID?: string;
  onApply: (patch: Partial<QuestionFormState>) => void;
}) {
  const [analysis, setAnalysis] = useState<AIAnalysis | null>(null);
  const [applied, setApplied] = useState(false);
  const mutation = useMutation({
    mutationFn: () => analyzeDraft({
      subject: state.subject,
      grade: state.grade,
      question_type: state.question_type,
      difficulty: state.difficulty,
      tags: parseTags(state.tagsText),
      category: state.category ?? "",
      knowledge_points: parseTags(state.knowledgePointsText ?? ""),
      content: state.content,
      options: isChoiceType(state.question_type) ? state.options : [],
      answer: state.answer,
      explanation: state.explanation,
    }),
    onSuccess: (result) => {
      setAnalysis(result);
      setApplied(false);
    },
  });

  const canAnalyze = richTextHasContent(state.content);

  return (
    <section aria-labelledby="question-ai-title" className="rounded-lg border border-accent/25 bg-accent-soft/30 p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 id="question-ai-title" className="flex items-center gap-1.5 text-copy-13 font-medium text-neutral-9"><MagicWandIcon className="size-4 text-accent" />AI 助教</h2>
          <p className="mt-1 text-label-12 leading-5 text-neutral-7">分析这道草稿，给出分类、知识点、标签、难度与解析建议。</p>
        </div>
      </div>

      <Button type="button" size="sm" variant="secondary" className="mt-3 w-full" disabled={!canAnalyze || mutation.isPending} onClick={() => mutation.mutate()}>
        {mutation.isPending ? "正在分析…" : canAnalyze ? "分析当前草稿" : "请先填写题干"}
      </Button>

      {mutation.isError ? <div className="mt-3"><FormError error={mutation.error} /></div> : null}
      {analysis ? (
        <div className="mt-3 grid gap-2 rounded-md border border-border bg-neutral-1 p-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-full bg-accent-soft px-2 py-0.5 font-mono text-label-12 text-accent-strong">{analysis.provider}</span>
            <span className="text-label-12 text-neutral-7">置信度 {Math.round(analysis.confidence * 100)}%</span>
            {analysis.review_required ? <span className="rounded-full bg-warning-soft px-2 py-0.5 text-label-12 text-warning">待复核</span> : null}
          </div>
          <dl className="grid gap-1.5 text-label-12 leading-5">
            <div className="flex gap-2"><dt className="w-14 shrink-0 text-neutral-6">分类</dt><dd className="text-neutral-9">{analysis.category || "—"}</dd></div>
            <div className="flex gap-2"><dt className="w-14 shrink-0 text-neutral-6">知识点</dt><dd className="text-neutral-9">{analysis.knowledge_points.join("、") || "—"}</dd></div>
            <div className="flex gap-2"><dt className="w-14 shrink-0 text-neutral-6">标签</dt><dd className="text-neutral-9">{analysis.tags.join("、") || "—"}</dd></div>
            <div className="flex gap-2"><dt className="w-14 shrink-0 text-neutral-6">难度</dt><dd className="text-neutral-9">{analysis.difficulty} / 5</dd></div>
          </dl>
          {analysis.warnings.length ? <ul className="grid gap-1 text-label-12 leading-5 text-warning">{analysis.warnings.map((warning) => <li key={warning}>· {warning}</li>)}</ul> : null}
          <Button
            type="button"
            size="sm"
            className="mt-1 w-full"
            disabled={applied}
            onClick={() => {
              onApply(buildPatch(state, analysis));
              setApplied(true);
            }}
          >
            {applied ? <><CheckIcon />已写入表单</> : "应用建议到表单"}
          </Button>
          <p className="text-caption-10 leading-4 text-neutral-6">只补充空白的答案与解析，不会覆盖已填写内容。</p>
        </div>
      ) : null}
      <Link
        href={`/ai?prompt=${encodeURIComponent(questionID
          ? `帮我深入分析题目 ${questionID}：检查答案解析、知识点、难度与相似题，必要时生成草稿修改建议。`
          : "帮我检查这道新题：分析题干、答案与解析，并给出知识点、难度和标签建议。")}`}
        className="mt-3 inline-block text-label-12 font-medium text-accent-strong underline-offset-2 hover:underline"
      >
        在 AI 助手中深入分析
      </Link>
    </section>
  );
}
