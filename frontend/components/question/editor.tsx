"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { renderToHTMLString } from "@tiptap/static-renderer";
import { ArrowLeftIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { createQuestion, questionQueryKeys, updateQuestion } from "@/lib/questions";
import { createQuestionFormState, isChoiceType, toQuestionInput, type EditableQuestionField, type QuestionFormState } from "@/lib/question-form";
import { normalizeChoiceAnswer, questionInputSchema, questionStatusLabels, questionTypeLabels, type Question, type QuestionInput } from "@/lib/question-schemas";
import { ocrToEditorJSON } from "@/lib/ocr-to-editor";
import { sanitizeRichText } from "@/lib/rich-text";
import { Button } from "@/components/ui/button";
import { FormError } from "@/components/auth-form";
import { questionEditorExtensions } from "./math-extension";
import { AnswerField, ChoiceOptionsField, QuestionMetadataFields } from "./question-fields";
import { RichTextEditor } from "./rich-text-editor";
import { OcrUpload } from "./ocr-upload";

export function QuestionEditor({ user, question, onSaved, onCancel }: {
  user: CurrentUser;
  question?: Question;
  onSaved: (message: string) => void;
  onCancel: () => void;
}) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<QuestionFormState>(() => createQuestionFormState(user, question));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [activeField, setActiveField] = useState<EditableQuestionField>("content");
  const [isDirty, setIsDirty] = useState(false);

  const saveMutation = useMutation({
    mutationFn: (input: QuestionInput) => question ? updateQuestion(question.id, input) : createQuestion(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: questionQueryKeys.all });
      setIsDirty(false);
      onSaved(question ? "草稿修改已保存" : "新草稿已保存");
    },
  });

  function updateForm(patch: Partial<QuestionFormState>) {
    setForm((current) => ({ ...current, ...patch }));
    setIsDirty(true);
  }

  function cancelEditing() {
    if (isDirty && !window.confirm("当前草稿尚未保存，确定放弃修改吗？")) return;
    onCancel();
  }

  function saveDraft(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const result = questionInputSchema.safeParse(toQuestionInput(form));
    if (!result.success) {
      const nextErrors: Record<string, string> = {};
      result.error.issues.forEach((issue) => {
        const path = issue.path.join(".") || "form";
        if (!nextErrors[path]) nextErrors[path] = issue.message;
      });
      setErrors(nextErrors);
      return;
    }
    setErrors({});
    saveMutation.mutate(result.data);
  }

  function applyOcrText(text: string): string | null {
    if (activeField === "answer" && isChoiceType(form.question_type)) {
      const normalized = normalizeChoiceAnswer(text);
      const validKeys = form.options.map((option) => option.key);
      const maximum = form.question_type === "single_choice" ? 1 : 4;
      if (normalized.length < 1 || normalized.length > maximum || normalized.split("").some((key) => !validKeys.includes(key as typeof validKeys[number]))) {
        return form.question_type === "single_choice" ? "请在校对文本中保留一个有效选项键，例如 A" : "请在校对文本中保留有效选项键，例如 A 或 AB";
      }
      updateForm({ answer: normalized });
      return null;
    }
    if (activeField === "answer" && form.question_type === "true_false") {
      const normalized = text.trim().toLowerCase();
      if (["true", "正确", "对"].includes(normalized)) { updateForm({ answer: "true" }); return null; }
      if (["false", "错误", "错"].includes(normalized)) { updateForm({ answer: "false" }); return null; }
      return "判断题答案请校对为“正确”或“错误”后再写入";
    }

    const html = sanitizeRichText(renderToHTMLString({ content: ocrToEditorJSON(text), extensions: questionEditorExtensions }));
    updateForm({ [activeField]: html });
    return null;
  }

  return (
    <form onSubmit={saveDraft} noValidate className="grid gap-6">
      <section className="border-b border-border pb-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <Button type="button" variant="ghost" size="sm" className="-ml-3 mb-3" onClick={cancelEditing}><ArrowLeftIcon />返回题目列表</Button>
            <div className="flex flex-wrap items-center gap-3">
              <h2 className="text-2xl font-semibold tracking-tight text-slate-950">{question ? "编辑自己的草稿" : "新建题目草稿"}</h2>
              <span className="rounded-full bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-800">{questionStatusLabels[question?.status ?? "draft"]}</span>
            </div>
            <p className="mt-2 text-sm leading-6 text-slate-600">{questionTypeLabels[form.question_type]} · 教师只能保存草稿，发布与归档由管理员处理。</p>
          </div>
          <Button type="submit" disabled={saveMutation.isPending}>{saveMutation.isPending ? "正在保存…" : "保存草稿"}</Button>
        </div>
      </section>

      {saveMutation.isError ? <FormError error={saveMutation.error} /> : null}
      {errors.form ? <p role="alert" className="text-sm text-red-600">{errors.form}</p> : null}
      <QuestionMetadataFields state={form} errors={errors} onChange={updateForm} />

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="grid min-w-0 gap-6">
          <RichTextEditor label="题干" value={form.content} error={errors.content} onChange={(content) => updateForm({ content })} onActivate={() => setActiveField("content")} />
          {isChoiceType(form.question_type) ? <ChoiceOptionsField options={form.options} errors={errors} onChange={(options) => {
            const validKeys = options.map((option) => option.key);
            updateForm({ options, answer: normalizeChoiceAnswer(form.answer).split("").filter((key) => validKeys.includes(key as typeof validKeys[number])).join("") });
          }} /> : null}
          <AnswerField type={form.question_type} options={form.options} answer={form.answer} error={errors.answer} onChange={(answer) => updateForm({ answer })} onActivate={() => setActiveField("answer")} />
          <RichTextEditor label="解析（可选）" value={form.explanation} error={errors.explanation} onChange={(explanation) => updateForm({ explanation })} onActivate={() => setActiveField("explanation")} />
        </div>
        <aside className="min-w-0 xl:sticky xl:top-24">
          <OcrUpload activeField={activeField} onActiveFieldChange={setActiveField} onApply={applyOcrText} />
        </aside>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-5">
        <p className="text-xs leading-5 text-slate-500">保存前会在浏览器再次移除脚本、图片和危险属性，服务端仍需执行白名单消毒。</p>
        <div className="flex gap-3">
          <Button type="button" variant="outline" disabled={saveMutation.isPending} onClick={cancelEditing}>取消</Button>
          <Button type="submit" disabled={saveMutation.isPending}>{saveMutation.isPending ? "正在保存…" : "保存草稿"}</Button>
        </div>
      </div>
    </form>
  );
}
