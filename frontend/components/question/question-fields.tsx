"use client";

import { MinusIcon, PlusIcon } from "@radix-ui/react-icons";
import { createChoiceOptions, type QuestionFormState } from "@/lib/question-form";
import { normalizeChoiceAnswer, questionTypeLabels, type QuestionOption, type QuestionType } from "@/lib/question-schemas";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { RichTextEditor } from "./rich-text-editor";

export function FieldError({ id, message }: { id: string; message?: string }) {
  return message ? <p id={id} className="text-xs leading-5 text-red-600">{message}</p> : null;
}

export function QuestionMetadataFields({ state, errors, onChange }: {
  state: QuestionFormState;
  errors: Record<string, string>;
  onChange: (patch: Partial<QuestionFormState>) => void;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
      <div className="grid gap-2">
        <Label htmlFor="question-subject">学科</Label>
        <Input id="question-subject" value={state.subject} readOnly aria-readonly="true" className="bg-slate-50 text-slate-600" aria-invalid={Boolean(errors.subject)} aria-describedby="question-subject-message" />
        <p id="question-subject-message" className={errors.subject ? "text-xs text-red-600" : "text-xs text-slate-500"}>{errors.subject ?? "须与教师账号学科一致"}</p>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="question-grade">年级</Label>
        <Input id="question-grade" value={state.grade} placeholder="例如：八年级" aria-invalid={Boolean(errors.grade)} aria-describedby="question-grade-error" onChange={(event) => onChange({ grade: event.target.value })} />
        <FieldError id="question-grade-error" message={errors.grade} />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="question-type">题型</Label>
        <Select id="question-type" value={state.question_type} onChange={(event) => {
          const questionType = event.target.value as QuestionType;
          const wasChoice = state.question_type === "single_choice" || state.question_type === "multiple_choice";
          const isChoice = questionType === "single_choice" || questionType === "multiple_choice";
          onChange({
            question_type: questionType,
            options: isChoice ? (wasChoice ? state.options : createChoiceOptions()) : [],
            answer: questionType === "single_choice" ? normalizeChoiceAnswer(state.answer).slice(0, 1) : "",
          });
        }}>
          {Object.entries(questionTypeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </Select>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="question-difficulty">难度</Label>
        <Select id="question-difficulty" value={state.difficulty} onChange={(event) => onChange({ difficulty: Number(event.target.value) })}>
          {[1, 2, 3, 4, 5].map((value) => <option key={value} value={value}>{value} 级</option>)}
        </Select>
      </div>
      <div className="grid gap-2 sm:col-span-2 xl:col-span-1">
        <Label htmlFor="question-tags">知识点标签</Label>
        <Input id="question-tags" value={state.tagsText} placeholder="方程；函数" aria-invalid={Boolean(errors.tags)} aria-describedby="question-tags-hint" onChange={(event) => onChange({ tagsText: event.target.value })} />
        <p id="question-tags-hint" className={errors.tags ? "text-xs text-red-600" : "text-xs text-slate-500"}>{errors.tags ?? "逗号或分号分隔，最多 10 个"}</p>
      </div>
    </div>
  );
}

function nextOptionKey(options: QuestionOption[]): QuestionOption["key"] {
  return (["A", "B", "C", "D"] as const).find((key) => !options.some((option) => option.key === key)) ?? "D";
}

export function ChoiceOptionsField({ options, errors, onChange }: {
  options: QuestionOption[];
  errors: Record<string, string>;
  onChange: (options: QuestionOption[]) => void;
}) {
  return (
    <section aria-labelledby="choice-options-title" className="grid gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h3 id="choice-options-title" className="text-sm font-semibold text-slate-800">选项</h3>
        <div className="flex gap-2">
          <Button type="button" variant="outline" size="sm" disabled={options.length <= 2} onClick={() => onChange(options.slice(0, -1))}><MinusIcon />删除末项</Button>
          <Button type="button" variant="outline" size="sm" disabled={options.length >= 4} onClick={() => onChange([...options, { key: nextOptionKey(options), content: "" }])}><PlusIcon />添加选项</Button>
        </div>
      </div>
      <FieldError id="choice-options-error" message={errors.options} />
      <div className="grid gap-4">
        {options.map((option, index) => (
          <RichTextEditor
            key={option.key}
            compact
            label={`选项 ${option.key}`}
            value={option.content}
            error={errors[`options.${index}.content`]}
            onChange={(content) => onChange(options.map((item) => item.key === option.key ? { ...item, content } : item))}
          />
        ))}
      </div>
    </section>
  );
}

export function AnswerField({ type, options, answer, error, onChange, onActivate }: {
  type: QuestionType;
  options: QuestionOption[];
  answer: string;
  error?: string;
  onChange: (answer: string) => void;
  onActivate: () => void;
}) {
  if (type === "fill_blank" || type === "essay") {
    return <RichTextEditor label={type === "fill_blank" ? "答案" : "参考答案"} value={answer} error={error} onChange={onChange} onActivate={onActivate} />;
  }

  const selected = normalizeChoiceAnswer(answer).split("");
  return (
    <fieldset className="grid gap-3" onFocus={onActivate}>
      <legend className="text-sm font-medium text-slate-700">答案</legend>
      <div className="flex flex-wrap gap-2" aria-describedby="question-answer-error">
        {type === "true_false" ? [
          { value: "true", label: "正确" },
          { value: "false", label: "错误" },
        ].map((item) => (
          <label key={item.value} className="flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border border-border bg-white px-4 text-sm has-checked:border-primary has-checked:bg-primary-soft has-checked:text-primary-strong">
            <input type="radio" name="true-false-answer" value={item.value} checked={answer === item.value} onChange={() => onChange(item.value)} className="accent-primary" />
            {item.label}
          </label>
        )) : options.map((option) => (
          <label key={option.key} className="flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border border-border bg-white px-4 text-sm has-checked:border-primary has-checked:bg-primary-soft has-checked:text-primary-strong">
            <input
              type={type === "single_choice" ? "radio" : "checkbox"}
              name={type === "single_choice" ? "single-choice-answer" : `multiple-choice-${option.key}`}
              checked={selected.includes(option.key)}
              onChange={(event) => {
                if (type === "single_choice") onChange(option.key);
                else onChange(normalizeChoiceAnswer([...selected.filter((key) => key !== option.key), ...(event.target.checked ? [option.key] : [])].join("")));
              }}
              className="accent-primary"
            />
            选项 {option.key}
          </label>
        ))}
      </div>
      {type === "multiple_choice" ? <p className="text-xs text-slate-500">至少选择一项，保存时按后端约定转换为逗号分隔的选项键。</p> : null}
      <FieldError id="question-answer-error" message={error} />
    </fieldset>
  );
}
