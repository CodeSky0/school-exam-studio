"use client";

import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DownloadIcon, UploadIcon } from "@radix-ui/react-icons";
import { ApiError } from "@/lib/api";
import { downloadQuestionImportTemplate, importQuestions, importValidationResultSchema, validateCsvFile, validateQuestionImport, type ImportValidationResult } from "@/lib/question-import";
import { questionTypeLabels } from "@/lib/question-schemas";
import { questionQueryKeys } from "@/lib/questions";
import { statsQueryKeys } from "@/lib/stats";
import { FormError } from "@/components/auth-form";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RichTextView } from "./rich-text-view";

function validationDetails(error: unknown): ImportValidationResult | null {
  if (!(error instanceof ApiError)) return null;
  const result = importValidationResultSchema.safeParse(error.details);
  return result.success ? result.data : null;
}

export function CsvImport({ onImported }: { onImported: (count: number) => void }) {
  const queryClient = useQueryClient();
  const inputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [validation, setValidation] = useState<ImportValidationResult | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);

  const validateMutation = useMutation({
    mutationFn: validateQuestionImport,
    onMutate: () => { setValidation(null); setLocalError(null); },
    onSuccess: setValidation,
    onError: (error) => setValidation(validationDetails(error)),
  });
  const importMutation = useMutation({
    mutationFn: importQuestions,
    onError: (error) => setValidation(validationDetails(error)),
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: questionQueryKeys.all }),
        queryClient.invalidateQueries({ queryKey: statsQueryKeys.teacher }),
      ]);
      onImported(result.imported);
      close();
    },
  });
  const downloadMutation = useMutation({
    mutationFn: downloadQuestionImportTemplate,
    onSuccess: (blob) => {
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = "questions-template.csv";
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
    },
  });

  const isPending = validateMutation.isPending || importMutation.isPending;
  const canImport = Boolean(file && validation && validation.errors.length === 0 && validation.valid_count > 0 && !isPending);
  const mutationError = validateMutation.isError ? validateMutation.error : importMutation.isError ? importMutation.error : null;

  function reset() {
    setFile(null);
    setValidation(null);
    setLocalError(null);
    validateMutation.reset();
    importMutation.reset();
    downloadMutation.reset();
    if (inputRef.current) inputRef.current.value = "";
  }

  function close() {
    setOpen(false);
    reset();
  }

  function chooseFile(selected: File | undefined) {
    setValidation(null);
    setLocalError(null);
    validateMutation.reset();
    importMutation.reset();
    if (!selected) { setFile(null); return; }
    const error = validateCsvFile(selected);
    if (error) {
      setFile(null);
      setLocalError(error);
      if (inputRef.current) inputRef.current.value = "";
      return;
    }
    setFile(selected);
  }

  return (
    <>
      <Button type="button" variant="outline" onClick={() => setOpen(true)}><UploadIcon />CSV 导入</Button>
      <Dialog
        open={open}
        title="CSV 导入题目"
        description="先校验并检查预览，再确认导入。所有题目将保存为当前教师的草稿。"
        onClose={close}
        isPending={isPending}
        className="max-w-6xl"
        footer={<>
          <Button type="button" variant="outline" disabled={isPending} onClick={close}>取消</Button>
          <Button type="button" variant="secondary" disabled={!file || isPending} onClick={() => { if (file) { importMutation.reset(); validateMutation.mutate(file); } }}>{validateMutation.isPending ? "正在校验…" : "校验并预览"}</Button>
          <Button type="button" disabled={!canImport} onClick={() => { if (file && canImport) { validateMutation.reset(); importMutation.mutate(file); } }}>{importMutation.isPending ? "正在导入…" : "确认导入"}</Button>
        </>}
      >
        <div className="grid gap-5">
          <div className="grid gap-2">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <Label htmlFor="question-csv-file">选择 CSV 文件</Label>
              <Button type="button" variant="ghost" size="sm" disabled={downloadMutation.isPending || isPending} onClick={() => downloadMutation.mutate()}><DownloadIcon />{downloadMutation.isPending ? "正在下载…" : "下载模板"}</Button>
            </div>
            <Input ref={inputRef} id="question-csv-file" type="file" accept=".csv,text/csv" disabled={isPending} className="h-auto cursor-pointer py-2 text-label-12 file:mr-3 file:rounded-md file:border-0 file:bg-accent-soft file:px-3 file:py-1.5 file:text-label-12 file:font-medium file:text-accent-strong" onChange={(event) => chooseFile(event.target.files?.[0])} />
            <p className="text-label-12 leading-5 text-neutral-7">UTF-8 编码，最多 5 MB、1,000 行。更换文件后必须重新校验。</p>
            {file ? <p className="break-all text-label-12 text-neutral-8">{file.name} · {(file.size / 1024).toFixed(1)} KB</p> : null}
          </div>
          {localError ? <p role="alert" className="rounded-lg border border-error-soft-strong bg-error-soft px-3.5 py-3 text-copy-13 text-error">{localError}</p> : null}
          {downloadMutation.isError ? <FormError error={downloadMutation.error} /> : null}
          {mutationError ? <FormError error={mutationError} /> : null}

          {validation ? (
            <section aria-label="CSV 校验结果" className="grid gap-4">
              <div className={validation.errors.length ? "rounded-lg border border-error-soft-strong bg-error-soft px-4 py-3 text-copy-13 text-error" : "rounded-lg border border-success/25 bg-success-soft px-4 py-3 text-copy-13 text-success"} role="status">
                {validation.errors.length ? `校验未通过：${validation.valid_count} 行有效，${validation.errors.length} 个错误。请修正文件后重新上传。` : `校验通过：共 ${validation.valid_count} 行可导入，以下为最多 50 行预览。`}
              </div>
              {validation.errors.length ? (
                <div className="overflow-x-auto rounded-lg border border-error-soft-strong" tabIndex={0} role="region" aria-label="CSV 逐行错误，可横向滚动">
                  <table className="w-full min-w-[600px] border-collapse text-left text-copy-13">
                    <thead className="bg-error-soft text-label-12 text-error"><tr><th className="px-3 py-2">CSV 行号</th><th className="px-3 py-2">字段</th><th className="px-3 py-2">错误说明</th></tr></thead>
                    <tbody className="divide-y divide-error-soft bg-neutral-1">{validation.errors.map((error, index) => <tr key={`${error.line}-${error.field}-${index}`}><td className="px-3 py-2 font-mono text-label-12">{error.line}</td><td className="px-3 py-2 text-label-12">{error.field || "整行"}</td><td className="px-3 py-2 text-label-12 leading-5 text-error">{error.message}</td></tr>)}</tbody>
                  </table>
                </div>
              ) : null}
              {validation.preview.length ? (
                <div className="overflow-x-auto rounded-lg border border-border" tabIndex={0} role="region" aria-label="CSV 校验预览，可横向滚动">
                  <table className="w-full min-w-[1000px] border-collapse text-left text-copy-13">
                    <thead className="bg-neutral-2 text-label-12 text-neutral-7"><tr><th className="px-3 py-2">行号</th><th className="px-3 py-2">题型</th><th className="px-3 py-2">学科 / 年级</th><th className="px-3 py-2">难度</th><th className="px-3 py-2">标签</th><th className="w-[34%] px-3 py-2">题干</th><th className="px-3 py-2">答案</th></tr></thead>
                    <tbody className="divide-y divide-border bg-neutral-1">{validation.preview.map((row, index) => <tr key={`${row.line ?? index + 2}-${index}`} className="align-top"><td className="px-3 py-3 font-mono text-label-12">{row.line ?? index + 2}</td><td className="whitespace-nowrap px-3 py-3 text-label-12">{questionTypeLabels[row.question_type]}<span className="mt-1 block text-neutral-6">{row.options.length} 个选项</span></td><td className="px-3 py-3 text-label-12">{row.subject}<span className="mt-1 block text-neutral-7">{row.grade}</span></td><td className="px-3 py-3 font-mono text-label-12">{row.difficulty} / 5</td><td className="max-w-32 px-3 py-3 text-label-12 leading-5">{row.tags.join("、") || "无"}</td><td className="max-w-md px-3 py-3"><RichTextView content={row.content} className="max-h-28 overflow-hidden text-label-12" /></td><td className="max-w-48 px-3 py-3"><RichTextView content={row.answer} className="max-h-28 overflow-hidden text-label-12" /></td></tr>)}</tbody>
                  </table>
                </div>
              ) : null}
            </section>
          ) : null}
          <p className="text-label-12 leading-5 text-neutral-7">确认导入会重新上传当前文件并由服务端再次完整校验；校验结果不等于已写入题库。</p>
        </div>
      </Dialog>
    </>
  );
}
