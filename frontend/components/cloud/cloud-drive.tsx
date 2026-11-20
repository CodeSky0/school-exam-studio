"use client";

import { useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ChevronRightIcon,
  DownloadIcon,
  FileTextIcon,
  HomeIcon,
  MagnifyingGlassIcon,
  PlusIcon,
  ReloadIcon,
  TrashIcon,
  UploadIcon,
} from "@radix-ui/react-icons";
import {
  CLOUD_ACCEPT,
  CLOUD_MAX_BYTES,
  cloudIndexHint,
  cloudIndexLabel,
  cloudIndexTone,
  cloudItemExtension,
  cloudKindLabel,
  cloudQueryKeys,
  createCloudFolder,
  deleteCloudItem,
  downloadCloudFile,
  formatCloudScore,
  listCloudItems,
  reindexAllCloudFiles,
  reindexCloudItem,
  searchCloudFiles,
  uploadCloudFile,
  type CloudItem,
  type CloudSearch,
} from "@/lib/cloud";
import { formatBytes } from "@/lib/ai-chat";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { StatusMessage } from "@/components/ui/status-message";

const CLOUD_EXTENSIONS = CLOUD_ACCEPT.split(",");

function FolderGlyph({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" className={className} aria-hidden="true">
      <path d="M3.5 6.5A1.5 1.5 0 0 1 5 5h4l2 2h8a1.5 1.5 0 0 1 1.5 1.5v9A1.5 1.5 0 0 1 19 19H5a1.5 1.5 0 0 1-1.5-1.5v-11Z" strokeLinejoin="round" />
    </svg>
  );
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "操作失败，请稍后重试";
}

function validateFile(file: File) {
  if (file.size > CLOUD_MAX_BYTES) return "文件超过 50 MiB 限制";
  const lower = file.name.toLowerCase();
  const matched = CLOUD_EXTENSIONS.some((ext) => (ext.startsWith(".") ? lower.endsWith(ext) : file.type === ext));
  if (!matched) return "仅支持 PDF、Word、PPT、Excel、TXT/Markdown/CSV 与图片";
  return "";
}

export function CloudDrive() {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [currentID, setCurrentID] = useState("");
  const [folderOpen, setFolderOpen] = useState(false);
  const [folderName, setFolderName] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<CloudItem | null>(null);
  const [actionError, setActionError] = useState("");
  const [notice, setNotice] = useState("");
  const [uploading, setUploading] = useState<{ done: number; total: number } | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResult, setSearchResult] = useState<CloudSearch | null>(null);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState("");

  const listQuery = useQuery({
    queryKey: cloudQueryKeys.list(currentID),
    queryFn: () => listCloudItems(currentID),
  });
  const items = useMemo(() => listQuery.data?.items ?? [], [listQuery.data]);
  const breadcrumbs = listQuery.data?.breadcrumbs ?? [];
  const parentID = breadcrumbs.length > 1 ? breadcrumbs[breadcrumbs.length - 2].id : "";
  const fileCount = useMemo(() => items.filter((item) => !item.is_folder).length, [items]);

  const createFolderMutation = useMutation({
    mutationFn: (name: string) => createCloudFolder(name, currentID),
    onSuccess: () => {
      setFolderOpen(false);
      setFolderName("");
      setActionError("");
      setNotice("文件夹已创建");
      void queryClient.invalidateQueries({ queryKey: cloudQueryKeys.list(currentID) });
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteCloudItem(id),
    onSuccess: (result) => {
      setDeleteTarget(null);
      setActionError("");
      setNotice(`已删除 ${result.deleted} 项`);
      void queryClient.invalidateQueries({ queryKey: cloudQueryKeys.list(currentID) });
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  const downloadMutation = useMutation({
    mutationFn: (item: CloudItem) => downloadCloudFile(item.id, item.name),
    onSuccess: () => setActionError(""),
    onError: (error) => setActionError(errorMessage(error)),
  });

  const reindexMutation = useMutation({
    mutationFn: (id: string) => reindexCloudItem(id),
    onSuccess: () => {
      setActionError("");
      setNotice("已加入向量化队列，稍后自动完成");
      void queryClient.invalidateQueries({ queryKey: cloudQueryKeys.list(currentID) });
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  const reindexAllMutation = useMutation({
    mutationFn: () => reindexAllCloudFiles(),
    onSuccess: (result) => {
      setActionError("");
      setNotice(`已加入向量化队列：${result.queued} 个文件`);
      void queryClient.invalidateQueries({ queryKey: cloudQueryKeys.list(currentID) });
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  async function handleSearch() {
    const query = searchQuery.trim();
    if (!query || searching) return;
    setSearchOpen(true);
    setSearching(true);
    setSearchError("");
    setSearchResult(null);
    try {
      setSearchResult(await searchCloudFiles(query, 12));
    } catch (error) {
      setSearchError(errorMessage(error));
    } finally {
      setSearching(false);
    }
  }

  async function handleFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    const selected = Array.from(files);
    const invalid = selected.find((file) => validateFile(file));
    if (invalid) {
      setActionError(`${invalid.name}：${validateFile(invalid)}`);
      return;
    }
    setActionError("");
    setNotice("");
    setUploading({ done: 0, total: selected.length });
    try {
      for (let index = 0; index < selected.length; index += 1) {
        await uploadCloudFile(selected[index], currentID);
        setUploading({ done: index + 1, total: selected.length });
      }
      setNotice(`已上传 ${selected.length} 个文件，正在后台自动向量化`);
    } catch (error) {
      setActionError(errorMessage(error));
    } finally {
      setUploading(null);
      await queryClient.invalidateQueries({ queryKey: cloudQueryKeys.list(currentID) });
    }
  }

  function openFolder(item: CloudItem) {
    setCurrentID(item.id);
    setActionError("");
    setNotice("");
  }

  return (
    <div className="motion-rise">
      <PageHeader
        eyebrow="共享资料库"
        title="云盘"
        description="教师与管理员共用的学校云盘。上传 PDF、Word、PPT 等资料后自动向量化，供 AI 助手语义检索与引用。"
        actions={(
          <>
            <Button type="button" variant="outline" onClick={() => { setFolderOpen(true); setActionError(""); }}>
              <PlusIcon />新建文件夹
            </Button>
            <Button type="button" onClick={() => fileInputRef.current?.click()} disabled={Boolean(uploading)}>
              <UploadIcon />{uploading ? `上传中 ${uploading.done}/${uploading.total}` : "上传文件"}
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept={CLOUD_ACCEPT}
              multiple
              className="hidden"
              aria-label="上传云盘文件"
              onChange={(event) => {
                void handleFiles(event.target.files);
                event.target.value = "";
              }}
            />
          </>
        )}
      />

      <form
        className="mb-4 flex flex-wrap items-center gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          void handleSearch();
        }}
      >
        <div className="relative min-w-0 flex-1">
          <MagnifyingGlassIcon className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-neutral-6" />
          <Input
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder="语义搜索云盘资料，例如：一元二次方程教案"
            className="pl-9"
            maxLength={200}
            aria-label="搜索云盘资料"
          />
        </div>
        <Button type="submit" disabled={searching || !searchQuery.trim()}>
          <MagnifyingGlassIcon />{searching ? "检索中…" : "检索"}
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={reindexAllMutation.isPending}
          onClick={() => reindexAllMutation.mutate()}
          title="重新解析并向量化本校全部云盘文件"
        >
          <ReloadIcon />{reindexAllMutation.isPending ? "排队中…" : "重建索引"}
        </Button>
      </form>

      <div className="mb-4 flex flex-wrap items-center gap-1.5 text-copy-13 text-neutral-7" aria-label="当前位置">
        <button type="button" onClick={() => setCurrentID("")} className={cn("inline-flex items-center gap-1 rounded px-1.5 py-0.5 transition-colors hover:bg-neutral-2 hover:text-neutral-10", currentID === "" && "text-neutral-10")}>
          <HomeIcon className="size-3.5" />全部文件
        </button>
        {breadcrumbs.map((crumb, index) => (
          <span key={crumb.id} className="inline-flex items-center gap-1.5">
            <ChevronRightIcon className="size-3 text-neutral-5" />
            <button
              type="button"
              onClick={() => setCurrentID(crumb.id)}
              className={cn("max-w-52 truncate rounded px-1.5 py-0.5 transition-colors hover:bg-neutral-2 hover:text-neutral-10", index === breadcrumbs.length - 1 && "font-medium text-neutral-10")}
            >
              {crumb.name}
            </button>
          </span>
        ))}
        <span className="ml-auto text-caption-10 text-neutral-6">{items.length} 项 · {fileCount} 个文件</span>
      </div>

      <StatusMessage message={actionError} tone="error" className="mb-4" />
      <StatusMessage message={notice} tone="success" className="mb-4" />

      <div className="overflow-hidden rounded-xl border border-border bg-paper shadow-sm">
        <div className="flex items-center justify-between border-b border-border bg-neutral-2/50 px-4 py-2.5 text-caption-10 uppercase tracking-[0.12em] text-neutral-6">
          <span>名称</span>
          <span>大小 / 操作</span>
        </div>
        {listQuery.isPending ? (
          <p className="px-4 py-12 text-center text-copy-13 text-neutral-6">正在读取云盘…</p>
        ) : listQuery.isError ? (
          <div className="px-4 py-12 text-center">
            <p className="text-copy-13 text-error">{errorMessage(listQuery.error)}</p>
            <Button type="button" variant="outline" size="sm" className="mt-4" onClick={() => { void listQuery.refetch(); }}>重试</Button>
          </div>
        ) : items.length === 0 ? (
          <div className="px-4 py-16 text-center">
            <FolderGlyph className="mx-auto size-10 text-neutral-4" />
            <p className="mt-3 text-copy-14 font-medium text-neutral-8">这里还没有文件</p>
            <p className="mt-1 text-copy-13 text-neutral-6">上传资料，或先创建一个文件夹。</p>
            <div className="mt-5 flex justify-center gap-3">
              <Button type="button" variant="outline" size="sm" onClick={() => setFolderOpen(true)}><PlusIcon />新建文件夹</Button>
              <Button type="button" size="sm" onClick={() => fileInputRef.current?.click()}><UploadIcon />上传文件</Button>
            </div>
          </div>
        ) : (
          <ul className="divide-y divide-border">
            {items.map((item) => (
              <li key={item.id} className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-neutral-2/40">
                <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-md border", item.is_folder ? "border-accent/20 bg-accent-soft text-accent-strong" : "border-border bg-neutral-2 text-neutral-7")}>
                  {item.is_folder ? <FolderGlyph className="size-5" /> : <FileTextIcon className="size-4" />}
                </span>
                <div className="min-w-0 flex-1">
                  {item.is_folder ? (
                    <button type="button" onClick={() => openFolder(item)} className="block max-w-full truncate text-left text-copy-13 font-medium text-neutral-9 hover:text-accent-strong">
                      {item.name}
                    </button>
                  ) : (
                    <span className="block max-w-full truncate text-copy-13 font-medium text-neutral-9">{item.name}</span>
                  )}
                  <span className="mt-0.5 block text-caption-10 text-neutral-6">
                    {cloudKindLabel(item)}{item.is_folder ? "" : ` · ${formatBytes(item.size_bytes)}`} · {cloudItemExtension(item) || "—"}
                  </span>
                  {!item.is_folder && cloudIndexLabel(item) ? (
                    <span className={cn("mt-0.5 flex flex-wrap items-center gap-1.5 text-caption-10", cloudIndexTone(item))}>
                      {cloudIndexLabel(item)}
                      {cloudIndexHint(item) ? <span className="max-w-64 truncate text-neutral-5" title={cloudIndexHint(item)}>· {cloudIndexHint(item)}</span> : null}
                    </span>
                  ) : null}
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  {!item.is_folder && (item.index_status === "failed" || item.index_status === "unsupported") ? (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={`重建 ${item.name} 的向量索引`}
                      title="重建向量索引"
                      disabled={reindexMutation.isPending}
                      onClick={() => reindexMutation.mutate(item.id)}
                    >
                      <ReloadIcon />
                    </Button>
                  ) : null}
                  {!item.is_folder ? (
                    <Button type="button" variant="ghost" size="icon" aria-label={`下载 ${item.name}`} title="下载" disabled={downloadMutation.isPending} onClick={() => downloadMutation.mutate(item)}>
                      <DownloadIcon />
                    </Button>
                  ) : (
                    <Button type="button" variant="ghost" size="icon" aria-label={`打开 ${item.name}`} title="打开" onClick={() => openFolder(item)}>
                      <ChevronRightIcon />
                    </Button>
                  )}
                  <Button type="button" variant="ghost" size="icon" aria-label={`删除 ${item.name}`} title="删除" onClick={() => { setDeleteTarget(item); setActionError(""); }}>
                    <TrashIcon />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      {breadcrumbs.length > 0 ? (
        <div className="mt-4">
          <Button type="button" variant="ghost" size="sm" onClick={() => setCurrentID(parentID)}>返回上一级</Button>
        </div>
      ) : null}

      <Dialog
        open={folderOpen}
        title="新建文件夹"
        description={currentID ? `将在「${breadcrumbs[breadcrumbs.length - 1]?.name ?? "当前文件夹"}」中创建。` : "将在云盘根目录创建。"}
        onClose={() => { if (!createFolderMutation.isPending) setFolderOpen(false); }}
        isPending={createFolderMutation.isPending}
        footer={(
          <>
            <Button type="button" variant="outline" disabled={createFolderMutation.isPending} onClick={() => setFolderOpen(false)}>取消</Button>
            <Button
              type="button"
              disabled={createFolderMutation.isPending || !folderName.trim()}
              onClick={() => createFolderMutation.mutate(folderName.trim())}
            >
              {createFolderMutation.isPending ? "创建中…" : "创建"}
            </Button>
          </>
        )}
      >
        <label className="block text-copy-13 font-medium text-neutral-8" htmlFor="cloud-folder-name">文件夹名称</label>
        <Input
          id="cloud-folder-name"
          value={folderName}
          maxLength={120}
          autoFocus
          placeholder="例如：高一数学资料"
          className="mt-2"
          onChange={(event) => setFolderName(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && folderName.trim() && !createFolderMutation.isPending) createFolderMutation.mutate(folderName.trim());
          }}
        />
      </Dialog>

      <Dialog
        open={searchOpen}
        title="云盘语义检索"
        description={searchResult ? `检索「${searchResult.query}」，扫描 ${searchResult.chunks_scanned} 个已索引片段` : "正在检索已向量化的云盘资料"}
        onClose={() => { if (!searching) setSearchOpen(false); }}
        isPending={searching}
        footer={(
          <Button type="button" variant="outline" disabled={searching} onClick={() => setSearchOpen(false)}>关闭</Button>
        )}
      >
        {searching ? (
          <p className="py-8 text-center text-copy-13 text-neutral-6">正在检索云盘向量索引…</p>
        ) : searchError ? (
          <p className="py-8 text-center text-copy-13 text-error">{searchError}</p>
        ) : !searchResult?.index_available ? (
          <p className="py-8 text-center text-copy-13 text-neutral-6">向量检索暂不可用，请确认后端已启用云盘索引。</p>
        ) : searchResult.hits.length === 0 ? (
          <p className="py-8 text-center text-copy-13 text-neutral-6">没有找到相关片段。可尝试更换关键词，或等待文件完成向量化。</p>
        ) : (
          <ul className="space-y-3">
            {searchResult.hits.map((hit) => (
              <li key={`${hit.item_id}-${hit.chunk_index}`} className="rounded-lg border border-border bg-neutral-1 px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <span className="flex min-w-0 items-center gap-2 text-copy-13 font-medium text-neutral-9">
                    <FileTextIcon className="size-3.5 shrink-0 text-neutral-6" />
                    <span className="truncate">{hit.name}</span>
                  </span>
                  <span className="shrink-0 text-caption-10 text-neutral-6">{formatCloudScore(hit.score)}</span>
                </div>
                <p className="mt-2 whitespace-pre-wrap text-copy-13 leading-6 text-neutral-7">{hit.excerpt}</p>
              </li>
            ))}
          </ul>
        )}
      </Dialog>

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title={deleteTarget?.is_folder ? "删除文件夹" : "删除文件"}
        description={deleteTarget?.is_folder
          ? `将删除「${deleteTarget.name}」及其中的全部内容，此操作不可恢复。`
          : `将删除「${deleteTarget?.name ?? ""}」，此操作不可恢复。`}
        confirmLabel="删除"
        isPending={deleteMutation.isPending}
        onConfirm={() => { if (deleteTarget) deleteMutation.mutate(deleteTarget.id); }}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
