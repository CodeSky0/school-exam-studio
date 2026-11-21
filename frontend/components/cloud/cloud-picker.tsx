"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { ChevronRightIcon, FileTextIcon, HomeIcon } from "@radix-ui/react-icons";
import { attachCloudFiles, type AttachmentRef } from "@/lib/ai-chat";
import {
  aiCloudFileIssue,
  cloudKindLabel,
  cloudQueryKeys,
  listCloudItems,
  type CloudItem,
} from "@/lib/cloud";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { StatusMessage } from "@/components/ui/status-message";

type CloudPickerProps = {
  open: boolean;
  maxSelectable: number;
  onClose: () => void;
  onAttached: (attachments: AttachmentRef[]) => void;
};

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "读取云盘失败，请稍后重试";
}

function FolderGlyph({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" className={className} aria-hidden="true">
      <path d="M3.5 6.5A1.5 1.5 0 0 1 5 5h4l2 2h8a1.5 1.5 0 0 1 1.5 1.5v9A1.5 1.5 0 0 1 19 19H5a1.5 1.5 0 0 1-1.5-1.5v-11Z" strokeLinejoin="round" />
    </svg>
  );
}

export function CloudPicker({ open, maxSelectable, onClose, onAttached }: CloudPickerProps) {
  const [folderID, setFolderID] = useState("");
  const [selected, setSelected] = useState<Map<string, CloudItem>>(new Map());
  const [actionError, setActionError] = useState("");

  const listQuery = useQuery({
    queryKey: cloudQueryKeys.list(folderID),
    queryFn: () => listCloudItems(folderID),
    enabled: open,
  });
  const items = listQuery.data?.items ?? [];
  const breadcrumbs = listQuery.data?.breadcrumbs ?? [];
  const selectedFiles = useMemo(() => Array.from(selected.values()), [selected]);
  const parentID = breadcrumbs.length > 1 ? breadcrumbs[breadcrumbs.length - 2].id : "";

  const attachMutation = useMutation({
    mutationFn: () => attachCloudFiles(selectedFiles.map((item) => item.id)),
    onSuccess: (result) => {
      setSelected(new Map());
      onAttached(result.items);
      onClose();
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  function toggle(item: CloudItem) {
    setActionError("");
    setSelected((current) => {
      const next = new Map(current);
      if (next.has(item.id)) {
        next.delete(item.id);
      } else {
        if (next.size >= maxSelectable) return current;
        next.set(item.id, item);
      }
      return next;
    });
  }

  function navigate(nextFolderID: string) {
    setFolderID(nextFolderID);
    setActionError("");
  }

  return (
    <Dialog
      open={open}
      title="从云盘选择"
      description={`选择可交给 AI 阅读的资料，最多 ${maxSelectable} 个。PPT、Excel 等格式请下载后使用。`}
      onClose={() => { if (!attachMutation.isPending) onClose(); }}
      isPending={attachMutation.isPending}
      className="sm:max-w-2xl"
      footer={(
        <>
          <span className="mr-auto text-copy-12 text-neutral-6" aria-live="polite">已选 {selectedFiles.length} / {maxSelectable}</span>
          <Button type="button" variant="outline" disabled={attachMutation.isPending} onClick={onClose}>取消</Button>
          <Button type="button" disabled={selectedFiles.length === 0 || attachMutation.isPending} onClick={() => attachMutation.mutate()}>
            {attachMutation.isPending ? "添加中…" : "添加到对话"}
          </Button>
        </>
      )}
    >
      <div className="mb-3 flex flex-wrap items-center gap-1 text-copy-12 text-neutral-7">
        <button type="button" onClick={() => navigate("")} className={cn("inline-flex items-center gap-1 rounded px-1.5 py-0.5 hover:bg-neutral-2 hover:text-neutral-10", folderID === "" && "text-neutral-10")}>
          <HomeIcon className="size-3.5" />全部文件
        </button>
        {breadcrumbs.map((crumb, index) => (
          <span key={crumb.id} className="inline-flex items-center gap-1">
            <ChevronRightIcon className="size-3 text-neutral-5" />
            <button type="button" onClick={() => navigate(crumb.id)} className={cn("max-w-40 truncate rounded px-1.5 py-0.5 hover:bg-neutral-2 hover:text-neutral-10", index === breadcrumbs.length - 1 && "font-medium text-neutral-10")}>
              {crumb.name}
            </button>
          </span>
        ))}
      </div>

      <StatusMessage message={actionError} tone="error" className="mb-3" />

      <div className="max-h-[52vh] overflow-y-auto rounded-lg border border-border">
        {listQuery.isPending ? (
          <p className="px-4 py-12 text-center text-copy-13 text-neutral-6">正在读取云盘…</p>
        ) : listQuery.isError ? (
          <div className="px-4 py-10 text-center">
            <p className="text-copy-13 text-error">{errorMessage(listQuery.error)}</p>
            <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => { void listQuery.refetch(); }}>重试</Button>
          </div>
        ) : items.length === 0 ? (
          <div className="px-4 py-12 text-center">
            <FolderGlyph className="mx-auto size-9 text-neutral-4" />
            <p className="mt-3 text-copy-13 text-neutral-7">这个文件夹是空的</p>
          </div>
        ) : (
          <ul className="divide-y divide-border">
            {items.map((item) => {
              const issue = aiCloudFileIssue(item);
              const selectable = !item.is_folder && !issue;
              const checked = selected.has(item.id);
              return (
                <li key={item.id}>
                  <button
                    type="button"
                    disabled={!item.is_folder && !selectable}
                    onClick={() => (item.is_folder ? navigate(item.id) : toggle(item))}
                    className={cn(
                      "flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors",
                      "hover:bg-neutral-2/50 disabled:cursor-not-allowed disabled:opacity-55",
                      checked && "bg-accent-soft/60",
                    )}
                  >
                    <span className={cn("flex size-8 shrink-0 items-center justify-center rounded-md border", item.is_folder ? "border-accent/20 bg-accent-soft text-accent-strong" : "border-border bg-neutral-2 text-neutral-7")}>
                      {item.is_folder ? <FolderGlyph className="size-4" /> : <FileTextIcon className="size-3.5" />}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-copy-13 font-medium text-neutral-9">{item.name}</span>
                      <span className="mt-0.5 block text-caption-10 text-neutral-6">
                        {cloudKindLabel(item)}
                        {issue ? ` · ${issue}` : ""}
                      </span>
                    </span>
                    {item.is_folder ? (
                      <ChevronRightIcon className="size-4 shrink-0 text-neutral-5" />
                    ) : (
                      <span className={cn("flex size-5 shrink-0 items-center justify-center rounded border", checked ? "border-accent bg-accent text-white" : "border-border bg-paper")} aria-hidden="true">
                        {checked ? <svg viewBox="0 0 12 12" className="size-3" fill="none" stroke="currentColor" strokeWidth="2"><path d="m2.5 6 2.2 2.2L9.5 3.5" strokeLinecap="round" strokeLinejoin="round" /></svg> : null}
                      </span>
                    )}
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </div>

      <div className="mt-3 flex items-center justify-between">
        <Button type="button" variant="ghost" size="sm" disabled={breadcrumbs.length === 0} onClick={() => navigate(parentID)}>返回上一级</Button>
        <span className="text-caption-10 text-neutral-6">支持 PDF、Word、文本与图片</span>
      </div>
    </Dialog>
  );
}
