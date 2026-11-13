"use client";

import { useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { UploadIcon } from "@radix-ui/react-icons";
import { recognizeQuestionImage, validateOcrFile } from "@/lib/ocr";
import type { EditableQuestionField } from "@/lib/question-form";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { FormError } from "@/components/auth-form";

const fieldLabels: Record<EditableQuestionField, string> = {
  content: "题干",
  answer: "答案",
  explanation: "解析",
};

export function OcrUpload({ activeField, onActiveFieldChange, onApply }: {
  activeField: EditableQuestionField;
  onActiveFieldChange: (field: EditableQuestionField) => void;
  onApply: (text: string) => string | null;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [resultText, setResultText] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const recognizeMutation = useMutation({
    mutationFn: recognizeQuestionImage,
    onSuccess: (result) => {
      if (!result.text.trim()) {
        setLocalError("识别结果为空，请重试或手动录入");
        return;
      }
      setResultText(result.text);
      setLocalError(null);
      setMessage("识别完成，请校对文本后写入当前字段");
      setFile(null);
      if (inputRef.current) inputRef.current.value = "";
    },
  });

  function chooseFile(selected: File | undefined) {
    setMessage(null);
    setLocalError(null);
    if (!selected) { setFile(null); return; }
    const validationError = validateOcrFile(selected);
    if (validationError) {
      setFile(null);
      setLocalError(validationError);
      if (inputRef.current) inputRef.current.value = "";
      return;
    }
    setFile(selected);
  }

  const error = localError ?? (recognizeMutation.isError ? recognizeMutation.error : null);

  return (
    <section aria-labelledby="ocr-panel-title" className="rounded-lg border border-border bg-neutral-1">
      <div className="border-b border-border px-4 py-3">
        <h2 id="ocr-panel-title" className="text-copy-16 font-medium text-neutral-10">图片识别辅助录入</h2>
        <p className="mt-1 text-label-12 leading-5 text-neutral-7">单张不超过 5 MB，支持 JPEG、PNG、WebP。图片不会写入题目。</p>
      </div>
      <div className="grid gap-4 p-4">
        <div className="grid gap-2">
          <Label htmlFor="ocr-target-field">识别结果写入字段</Label>
          <Select id="ocr-target-field" value={activeField} onChange={(event) => onActiveFieldChange(event.target.value as EditableQuestionField)}>
            {Object.entries(fieldLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </Select>
          <p className="text-label-12 leading-5 text-neutral-7">聚焦题干、答案或解析编辑器也会更新此选项。</p>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="ocr-image">题目截图</Label>
          <Input
            ref={inputRef}
            id="ocr-image"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            className="h-auto cursor-pointer py-2 text-label-12 file:mr-3 file:rounded-md file:border-0 file:bg-accent-soft file:px-3 file:py-1.5 file:text-label-12 file:font-medium file:text-accent-strong"
            onChange={(event) => chooseFile(event.target.files?.[0])}
          />
          {file ? <p className="break-all text-label-12 text-neutral-7">{file.name} · {(file.size / 1024 / 1024).toFixed(2)} MB</p> : null}
        </div>
        <Button
          type="button"
          variant="secondary"
          disabled={!file || recognizeMutation.isPending}
          onClick={() => {
            if (!file) return;
            const validationError = validateOcrFile(file);
            if (validationError) { setLocalError(validationError); return; }
            setLocalError(null);
            setMessage(null);
            recognizeMutation.mutate(file);
          }}
        >
          <UploadIcon />{recognizeMutation.isPending ? "正在识别…" : "开始识别"}
        </Button>
        {error ? <FormError error={error} /> : null}
        {message ? <p role="status" className="rounded-lg border border-accent-soft-strong bg-accent-soft px-3 py-2 text-label-12 leading-5 text-info">{message}</p> : null}
        <div className="grid gap-2">
          <Label htmlFor="ocr-result">识别结果（可校对）</Label>
          <Textarea id="ocr-result" rows={8} value={resultText} placeholder="识别文本与 $…$、$$…$$ 公式将在这里显示" onChange={(event) => { setResultText(event.target.value); setMessage(null); }} />
        </div>
        <div className="grid grid-cols-2 gap-2">
          <Button
            type="button"
            disabled={!resultText.trim()}
            onClick={() => {
              const applyError = onApply(resultText);
              if (applyError) { setLocalError(applyError); setMessage(null); return; }
              setLocalError(null);
              setMessage(`已写入${fieldLabels[activeField]}，请继续校对并显式保存草稿`);
            }}
          >写入{fieldLabels[activeField]}</Button>
          <Button type="button" variant="outline" disabled={!resultText} onClick={() => { setResultText(""); setMessage(null); }}>清空结果</Button>
        </div>
        <p className="text-label-12 leading-5 text-neutral-7">写入会替换所选字段的当前内容。OCR 不会创建题目，保存前仍可继续编辑。</p>
      </div>
    </section>
  );
}
