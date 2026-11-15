"use client";

import { useState } from "react";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import { CaretDownIcon, CaretRightIcon, ChevronLeftIcon, ChevronRightIcon, Pencil1Icon, TrashIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { normalizeChoiceAnswer, questionStatusLabels, questionTypeLabels, type Question, type QuestionList } from "@/lib/question-schemas";
import { cn } from "@/lib/utils";
import { FormError } from "@/components/auth-form";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { RichTextView } from "./rich-text-view";

const statusStyles = {
  draft: "border-warning/30 bg-warning-soft text-warning",
  published: "border-success/25 bg-success-soft text-success",
  archived: "border-neutral-4 bg-neutral-3 text-neutral-7",
};

const listVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { staggerChildren: 0.035 } },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  visible: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -6 },
};

const detailVariants: Variants = {
  hidden: { opacity: 0, y: -6 },
  visible: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -6 },
};

function formatUpdatedAt(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false, timeZone: "Asia/Shanghai" }).format(date);
}

function QuestionAnswer({ question }: { question: Question }) {
  if (question.question_type === "true_false") return <span>{question.answer === "true" ? "正确" : "错误"}</span>;
  if (question.question_type === "single_choice" || question.question_type === "multiple_choice") {
    return <span className="font-mono font-medium text-accent-strong">{normalizeChoiceAnswer(question.answer).split("").join("、")}</span>;
  }
  return <RichTextView content={question.answer} />;
}

function QuestionDetails({ question }: { question: Question }) {
  return (
    <div className="grid gap-5 border-t border-accent-soft-strong bg-neutral-2/70 px-5 py-5 lg:grid-cols-2">
      <section>
        <h3 className="text-label-12 font-medium tracking-wide text-neutral-7">完整题干</h3>
        <div className="mt-2 rounded-lg border border-border bg-neutral-1 p-4"><RichTextView content={question.content} /></div>
        {question.options.length > 0 ? (
          <div className="mt-4 grid gap-2">
            {question.options.map((option) => (
              <div key={option.key} className="grid grid-cols-[1.75rem_minmax(0,1fr)] items-start gap-2 rounded-lg border border-border bg-neutral-1 px-3 py-2">
                <span className="font-mono text-copy-13 font-medium text-accent-strong">{option.key}.</span>
                <RichTextView content={option.content} />
              </div>
            ))}
          </div>
        ) : null}
      </section>
      <section className="grid content-start gap-4">
        <div>
          <h3 className="text-label-12 font-medium tracking-wide text-neutral-7">答案</h3>
          <div className="mt-2 rounded-lg border border-border bg-neutral-1 p-4 text-copy-13"><QuestionAnswer question={question} /></div>
        </div>
        <div>
          <h3 className="text-label-12 font-medium tracking-wide text-neutral-7">解析</h3>
          <div className="mt-2 rounded-lg border border-border bg-neutral-1 p-4"><RichTextView content={question.explanation} /></div>
        </div>
        <dl className="grid grid-cols-2 gap-3 text-label-12 text-neutral-7">
          <div><dt>分类</dt><dd className="mt-1 text-neutral-8">{question.category || "未分类"}</dd></div>
          <div><dt>知识点</dt><dd className="mt-1 text-neutral-8">{question.knowledge_points.join("、") || "未标注"}</dd></div>
          <div><dt>题目 ID</dt><dd className="mt-1 break-all font-mono text-neutral-8">{question.id}</dd></div>
          <div><dt>创建时间</dt><dd className="mt-1 text-neutral-8">{formatUpdatedAt(question.created_at)}</dd></div>
          <AIMeta question={question} />
        </dl>
      </section>
    </div>
  );
}

function TableSkeleton() {
  return (
    <div className="grid animate-pulse gap-px bg-border" aria-label="正在读取题目列表">
      {Array.from({ length: 5 }, (_, index) => <div key={index} className="h-20 bg-neutral-1" />)}
    </div>
  );
}

function QuestionStatus({ question }: { question: Question }) {
  return <span className={cn("inline-flex rounded-full border px-2 py-0.5 text-label-12 font-medium", statusStyles[question.status])}>{questionStatusLabels[question.status]}</span>;
}

function AIQuestionBadge({ question }: { question: Question }) {
  if (!question.ai_generated) return null;
  return <span className="inline-flex rounded-full border border-accent/30 bg-accent-soft px-2 py-0.5 text-label-12 font-medium text-accent-strong">AI 生成</span>;
}

function AIMeta({ question }: { question: Question }) {
  if (!question.ai_generated) return null;
  const rationale = typeof question.ai_meta.rationale === "string" ? question.ai_meta.rationale : "";
  const provider = typeof question.ai_meta.provider === "string" ? question.ai_meta.provider : "";
  return (
    <div className="col-span-2">
      <dt>AI 元信息</dt>
      <dd className="mt-1 leading-5 text-neutral-8">
        {[provider, rationale].filter(Boolean).join(" · ") || "AI 生成内容"}
        {question.ai_meta.review_required === true ? " · 待人工复核" : ""}
      </dd>
    </div>
  );
}

function QuestionActions({ canEdit, isDeleting, onEdit, onDelete, renderActions, className }: {
  canEdit: boolean;
  isDeleting: boolean;
  onEdit: () => void;
  onDelete: () => void;
  renderActions?: () => React.ReactNode;
  className?: string;
}) {
  if (renderActions) return renderActions();
  return (
    <div className={cn("flex flex-wrap justify-end gap-1", className)}>
      {canEdit ? <><Button type="button" variant="ghost" size="sm" onClick={onEdit}><Pencil1Icon />编辑</Button><Button type="button" variant="ghost" size="sm" className="text-error hover:bg-error-soft hover:text-error" disabled={isDeleting} onClick={onDelete}><TrashIcon />{isDeleting ? "删除中" : "删除"}</Button></> : <span className="text-copy-13 text-neutral-6">仅查看</span>}
    </div>
  );
}

export function QuestionTable({ user, data, isPending, isFetching, error, deletingId, onRetry, onEdit, onDelete, onPageChange, onPageSizeChange, renderActions, emptyDescription }: {
  user: CurrentUser;
  data?: QuestionList;
  isPending: boolean;
  isFetching: boolean;
  error: unknown;
  deletingId?: string;
  onRetry: () => void;
  onEdit?: (question: Question) => void;
  onDelete?: (question: Question) => void;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  renderActions?: (question: Question) => React.ReactNode;
  emptyDescription?: string;
}) {
  const [expandedId, setExpandedId] = useState<string | null>(null);
  if (isPending) return <TableSkeleton />;
  if (error) return <div className="border-b border-border bg-neutral-1 p-5"><FormError error={error} /><Button variant="outline" size="sm" className="mt-3" onClick={onRetry}>重新读取</Button></div>;
  if (!data || data.items.length === 0) return <div className="border-b border-border bg-neutral-1 px-5 py-14 text-center"><h2 className="text-copy-13 font-medium text-neutral-10">当前筛选下没有题目</h2><p className="mt-2 text-copy-13 text-neutral-7">{emptyDescription ?? "调整筛选条件，或新建一道草稿。"}</p></div>;

  const totalPages = Math.max(1, Math.ceil(data.total / data.page_size));
  return (
    <div aria-busy={isFetching}>
      <div className="flex min-h-9 items-center justify-between px-4 text-label-12 text-neutral-7">
        <span>共 {data.total} 道题{isFetching ? " · 正在更新列表…" : ""}</span>
        <span>每页最多 100 条</span>
      </div>
      <div className="hidden overflow-x-auto border-y border-border md:block" tabIndex={0} role="region" aria-label="题目列表，可横向滚动">
        <table className="w-full min-w-[1120px] border-collapse bg-neutral-1 text-left text-copy-13">
          <thead className="bg-neutral-2 text-label-12 font-medium text-neutral-7">
            <tr className="border-b border-border">
              <th scope="col" className="w-[31%] px-4 py-3">题目</th><th scope="col" className="px-3 py-3">学科 / 年级</th><th scope="col" className="px-3 py-3">题型</th><th scope="col" className="px-3 py-3">难度</th><th scope="col" className="px-3 py-3">标签</th><th scope="col" className="px-3 py-3">创建人</th><th scope="col" className="px-3 py-3">状态</th><th scope="col" className="px-3 py-3">更新时间</th><th scope="col" className="px-4 py-3 text-right">操作</th>
            </tr>
          </thead>
          <motion.tbody className="divide-y divide-border" variants={listVariants} initial="hidden" animate="visible">
            {data.items.map((question) => {
              const expanded = expandedId === question.id;
              return (
                <QuestionRow
                  key={question.id}
                  question={question}
                  canEdit={question.creator_id === user.id && question.status === "draft"}
                  expanded={expanded}
                  isDeleting={deletingId === question.id}
                  onToggle={() => setExpandedId(expanded ? null : question.id)}
                  onEdit={() => onEdit?.(question)}
                  onDelete={() => onDelete?.(question)}
                  renderActions={renderActions ? () => renderActions(question) : undefined}
                />
              );
            })}
          </motion.tbody>
        </table>
      </div>
      <motion.div className="border-y border-border md:hidden" variants={listVariants} initial="hidden" animate="visible" aria-label="题目摘要列表">
        <AnimatePresence initial={false} mode="popLayout">
          {data.items.map((question) => {
            const expanded = expandedId === question.id;
            return (
              <QuestionMobileCard
                key={question.id}
                question={question}
                canEdit={question.creator_id === user.id && question.status === "draft"}
                expanded={expanded}
                isDeleting={deletingId === question.id}
                onToggle={() => setExpandedId(expanded ? null : question.id)}
                onEdit={() => onEdit?.(question)}
                onDelete={() => onDelete?.(question)}
                renderActions={renderActions ? () => renderActions(question) : undefined}
              />
            );
          })}
        </AnimatePresence>
      </motion.div>
      <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-4">
        <div className="flex items-center gap-2">
          <Label htmlFor="question-page-size" className="text-label-12">每页</Label>
          <Select id="question-page-size" className="h-8 w-20" value={data.page_size} onChange={(event) => onPageSizeChange(Number(event.target.value))}><option value={10}>10</option><option value={20}>20</option><option value={50}>50</option></Select>
        </div>
        <div className="flex items-center gap-3">
          <span className="font-mono text-label-12 text-neutral-7">第 {data.page} / {totalPages} 页</span>
          <Button variant="outline" size="sm" disabled={data.page <= 1} onClick={() => onPageChange(data.page - 1)}><ChevronLeftIcon />上一页</Button>
          <Button variant="outline" size="sm" disabled={data.page >= totalPages} onClick={() => onPageChange(data.page + 1)}>下一页<ChevronRightIcon /></Button>
        </div>
      </div>
    </div>
  );
}

interface QuestionItemProps {
  question: Question;
  canEdit: boolean;
  expanded: boolean;
  isDeleting: boolean;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
  renderActions?: () => React.ReactNode;
}

function QuestionRow({ question, canEdit, expanded, isDeleting, onToggle, onEdit, onDelete, renderActions }: QuestionItemProps) {
  const detailsId = `desktop-question-details-${question.id}`;
  return (
    <>
      <motion.tr layout="position" variants={itemVariants} className={cn("align-top transition-colors", expanded ? "bg-accent-soft/40" : "hover:bg-neutral-2/70")}>
        <td className="px-4 py-4">
          <Button type="button" variant="ghost" size="sm" className="mb-2 -ml-2 h-7 px-2 text-label-12" aria-expanded={expanded} aria-controls={detailsId} onClick={onToggle}>{expanded ? <CaretDownIcon /> : <CaretRightIcon />}{expanded ? "收起" : "查看"}</Button>
          <RichTextView content={question.content} className="max-h-20 overflow-hidden text-copy-13 leading-5" />
        </td>
        <td className="px-3 py-4 text-copy-13 leading-5 text-neutral-8">{question.subject}<br /><span className="text-neutral-7">{question.grade}</span>{question.category ? <span className="mt-1 block text-label-12 text-neutral-6">{question.category}</span> : null}</td>
        <td className="whitespace-nowrap px-3 py-4 text-copy-13">{questionTypeLabels[question.question_type]}</td>
        <td className="px-3 py-4 font-mono text-copy-13">{question.difficulty} / 5</td>
        <td className="max-w-44 px-3 py-4"><div className="flex flex-wrap gap-1">{question.knowledge_points.map((point) => <span key={point} className="rounded bg-accent-soft px-1.5 py-0.5 text-label-12 text-accent-strong">{point}</span>)}{question.tags.map((tag) => <span key={tag} className="rounded bg-neutral-3 px-1.5 py-0.5 text-label-12 text-neutral-7">{tag}</span>)}{question.knowledge_points.length === 0 && question.tags.length === 0 ? <span className="text-copy-13 text-neutral-6">无</span> : null}</div></td>
        <td className="px-3 py-4 text-copy-13 text-neutral-8">{question.creator_name}</td>
        <td className="px-3 py-4"><div className="flex flex-wrap items-center gap-1"><QuestionStatus question={question} /><AIQuestionBadge question={question} /></div></td>
        <td className="whitespace-nowrap px-3 py-4 text-copy-13 text-neutral-7"><time dateTime={question.updated_at}>{formatUpdatedAt(question.updated_at)}</time></td>
        <td className="px-4 py-4"><QuestionActions canEdit={canEdit} isDeleting={isDeleting} onEdit={onEdit} onDelete={onDelete} renderActions={renderActions} /></td>
      </motion.tr>
      <AnimatePresence initial={false}>
        {expanded ? <motion.tr key={detailsId} variants={detailVariants} initial="hidden" animate="visible" exit="exit"><td colSpan={9} className="p-0" id={detailsId}><QuestionDetails question={question} /></td></motion.tr> : null}
      </AnimatePresence>
    </>
  );
}

function QuestionMobileCard({ question, canEdit, expanded, isDeleting, onToggle, onEdit, onDelete, renderActions }: QuestionItemProps) {
  const detailsId = `mobile-question-details-${question.id}`;
  return (
    <motion.article layout="position" variants={itemVariants} exit="exit" className="border-b border-border p-4 last:border-b-0">
      <div className="flex items-start justify-between gap-3">
        <Button type="button" variant="ghost" size="sm" className="-ml-2 h-8 px-2 text-label-12" aria-expanded={expanded} aria-controls={detailsId} onClick={onToggle}>{expanded ? <CaretDownIcon /> : <CaretRightIcon />}{expanded ? "收起详情" : "查看详情"}</Button>
        <div className="flex flex-wrap justify-end gap-1"><QuestionStatus question={question} /><AIQuestionBadge question={question} /></div>
      </div>
      <RichTextView content={question.content} className="mt-2 max-h-24 overflow-hidden text-copy-14 leading-6" />
      <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 text-label-12">
        <div><dt className="text-neutral-6">学科 / 年级</dt><dd className="mt-1 text-copy-13 text-neutral-9">{question.subject} · {question.grade}</dd></div>
        <div><dt className="text-neutral-6">题型 / 难度</dt><dd className="mt-1 text-copy-13 text-neutral-9">{questionTypeLabels[question.question_type]} · {question.difficulty} / 5</dd></div>
        <div><dt className="text-neutral-6">分类</dt><dd className="mt-1 text-copy-13 text-neutral-9">{question.category || "未分类"}</dd></div>
        <div><dt className="text-neutral-6">知识点</dt><dd className="mt-1 text-copy-13 text-neutral-9">{question.knowledge_points.join("、") || "未标注"}</dd></div>
        <div><dt className="text-neutral-6">创建人</dt><dd className="mt-1 text-copy-13 text-neutral-9">{question.creator_name}</dd></div>
        <div><dt className="text-neutral-6">更新时间</dt><dd className="mt-1 text-copy-13 text-neutral-9"><time dateTime={question.updated_at}>{formatUpdatedAt(question.updated_at)}</time></dd></div>
      </dl>
      <div className="mt-3 flex flex-wrap gap-1">{question.knowledge_points.map((point) => <span key={point} className="rounded bg-accent-soft px-1.5 py-0.5 text-label-12 text-accent-strong">{point}</span>)}{question.tags.map((tag) => <span key={tag} className="rounded bg-neutral-3 px-1.5 py-0.5 text-label-12 text-neutral-7">{tag}</span>)}</div>
      <div className="mt-4 border-t border-border pt-3"><QuestionActions canEdit={canEdit} isDeleting={isDeleting} onEdit={onEdit} onDelete={onDelete} renderActions={renderActions} className="justify-start" /></div>
      <AnimatePresence initial={false}>
        {expanded ? <motion.div key={detailsId} id={detailsId} variants={detailVariants} initial="hidden" animate="visible" exit="exit" className="mt-4 overflow-hidden rounded-lg"><QuestionDetails question={question} /></motion.div> : null}
      </AnimatePresence>
    </motion.article>
  );
}
