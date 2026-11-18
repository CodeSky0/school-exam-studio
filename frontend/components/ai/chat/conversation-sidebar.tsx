"use client";

import { useState } from "react";
import {
  ChatBubbleIcon,
  MagnifyingGlassIcon,
  Pencil1Icon,
  PlusIcon,
  StarFilledIcon,
  StarIcon,
  TrashIcon,
} from "@radix-ui/react-icons";
import { formatChatTime, type Conversation } from "@/lib/ai-chat";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Input } from "@/components/ui/input";

interface ConversationSidebarProps {
  conversations: Conversation[];
  activeID: string;
  isLoading: boolean;
  error: string;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onRename: (id: string, title: string) => void;
  onTogglePin: (conversation: Conversation) => void;
  onDelete: (id: string) => void;
  onDeleteMany: (ids: string[]) => Promise<void>;
}

export function ConversationSidebar({
  conversations, activeID, isLoading, error, onSelect, onCreate, onRename, onTogglePin, onDelete, onDeleteMany,
}: ConversationSidebarProps) {
  const [search, setSearch] = useState("");
  const [renamingID, setRenamingID] = useState("");
  const [renameValue, setRenameValue] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<Conversation | null>(null);
  const [managing, setManaging] = useState(false);
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchError, setBatchError] = useState("");

  const filtered = search.trim()
    ? conversations.filter((item) => item.title.toLowerCase().includes(search.trim().toLowerCase()))
    : conversations;

  function startRename(conversation: Conversation) {
    setRenamingID(conversation.id);
    setRenameValue(conversation.title);
  }

  function toggleSelected(id: string) {
    setSelectedIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  }

  function toggleSelectAll() {
    const allSelected = filtered.length > 0 && filtered.every((item) => selectedIDs.includes(item.id));
    setSelectedIDs(allSelected ? [] : filtered.map((item) => item.id));
  }

  async function confirmBatchDelete() {
    setBatchDeleting(true);
    setBatchError("");
    try {
      await onDeleteMany(selectedIDs);
      setSelectedIDs([]);
      setManaging(false);
      setBatchDeleteOpen(false);
    } catch (caught) {
      setBatchError(caught instanceof Error ? caught.message : "删除失败，请稍后重试");
    } finally {
      setBatchDeleting(false);
    }
  }

  function commitRename() {
    if (renamingID && renameValue.trim()) {
      onRename(renamingID, renameValue.trim());
    }
    setRenamingID("");
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-neutral-2/55">
      <div className="border-b border-border px-3 py-3">
        <div className="flex items-center justify-between gap-2">
          <p className="text-caption-10 font-medium tracking-[0.14em] text-neutral-6">
            对话历史<span className="ml-1.5 font-normal text-neutral-5">{conversations.length}</span>
          </p>
          <div className="flex items-center gap-1">
            <Button
              type="button"
              size="icon"
              variant={managing ? "secondary" : "ghost"}
              aria-label={managing ? "退出批量管理" : "批量管理对话"}
              title={managing ? "退出批量管理" : "批量管理对话"}
              onClick={() => { setManaging(!managing); setSelectedIDs([]); setBatchError(""); }}
            >
              <TrashIcon />
            </Button>
            <Button type="button" size="icon" variant="secondary" aria-label="新建对话" onClick={onCreate}>
              <PlusIcon />
            </Button>
          </div>
        </div>
        {managing ? (
          <div className="mt-2 flex items-center justify-between gap-2 rounded-md border border-accent/25 bg-accent-soft/40 px-2 py-1.5">
            <span className="text-caption-10 font-medium text-accent-strong">已选 {selectedIDs.length} 个</span>
            <button type="button" className="text-caption-10 font-medium text-accent-strong hover:underline" onClick={toggleSelectAll}>
              {filtered.length > 0 && filtered.every((item) => selectedIDs.includes(item.id)) ? "取消全选" : "全选当前列表"}
            </button>
          </div>
        ) : null}
        <div className="mt-3 flex items-center gap-2 rounded-md border border-border bg-neutral-1 px-2">
          <MagnifyingGlassIcon className="size-3.5 text-neutral-6" />
          <input
            value={search}
            onChange={(event) => { setSearch(event.target.value); setSelectedIDs([]); }}
            placeholder="搜索会话"
            aria-label="搜索会话"
            className="h-8 min-w-0 flex-1 bg-transparent text-label-12 text-neutral-9 outline-none placeholder:text-neutral-6"
          />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-2 py-2">
        {isLoading ? <p className="px-2 py-3 text-label-12 text-neutral-7">正在读取会话…</p> : null}
        {error ? <p className="px-2 py-3 text-label-12 leading-5 text-error">{error}</p> : null}
        {!isLoading && !error && filtered.length === 0 ? (
          <div className="px-2 py-6 text-center">
            <ChatBubbleIcon className="mx-auto size-6 text-neutral-5" />
            <p className="mt-2 text-label-12 leading-5 text-neutral-6">{search ? "没有匹配的会话" : "暂无对话"}</p>
          </div>
        ) : null}
        <ul className="grid gap-1">
          {filtered.map((conversation) => {
            const active = conversation.id === activeID;
            const renaming = renamingID === conversation.id;
            return (
              <li key={conversation.id}>
                <div
                  className={cn(
                    "group relative rounded-md border transition-colors",
                    managing && selectedIDs.includes(conversation.id)
                      ? "border-accent/40 bg-accent-soft/35"
                      : active ? "border-accent/30 bg-neutral-1 shadow-sm" : "border-transparent hover:border-border hover:bg-neutral-1/70",
                  )}
                >
                  {renaming ? (
                    <div className="p-2">
                      <Input
                        autoFocus
                        value={renameValue}
                        maxLength={60}
                        aria-label="会话标题"
                        onChange={(event) => setRenameValue(event.target.value)}
                        onKeyDown={(event) => {
                          if (event.key === "Enter") commitRename();
                          if (event.key === "Escape") setRenamingID("");
                        }}
                        onBlur={commitRename}
                        className="h-8 text-label-12"
                      />
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={() => managing ? toggleSelected(conversation.id) : onSelect(conversation.id)}
                      className={cn("block w-full px-3 py-2.5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/35", managing ? "pr-3" : "pr-16")}
                      aria-current={active && !managing ? "true" : undefined}
                      aria-pressed={managing ? selectedIDs.includes(conversation.id) : undefined}
                    >
                      <span className="flex items-center gap-1.5">
                        {managing ? (
                          <input
                            type="checkbox"
                            checked={selectedIDs.includes(conversation.id)}
                            onChange={() => toggleSelected(conversation.id)}
                            onClick={(event) => event.stopPropagation()}
                            aria-label={`选择会话 ${conversation.title}`}
                            className="size-3.5 shrink-0"
                          />
                        ) : null}
                        {conversation.pinned ? <StarFilledIcon className="size-3 shrink-0 text-warning" aria-label="已置顶" /> : null}
                        <span className={cn("truncate text-label-12 font-medium", active ? "text-accent-strong" : "text-neutral-9")}>{conversation.title}</span>
                      </span>
                      <span className="mt-1 flex items-center gap-1.5 text-caption-10 text-neutral-6">
                        <span>{formatChatTime(conversation.last_message_at)}</span>
                        <span>·</span>
                        <span>{conversation.message_count} 条</span>
                      </span>
                    </button>
                  )}
                  {!renaming && !managing ? (
                    <div className="absolute right-1 top-1.5 flex opacity-0 transition-opacity focus-within:opacity-100 group-hover:opacity-100">
                      <button type="button" aria-label={conversation.pinned ? "取消置顶" : "置顶会话"} onClick={() => onTogglePin(conversation)} className="rounded p-1 text-neutral-6 hover:bg-neutral-3 hover:text-warning">
                        {conversation.pinned ? <StarFilledIcon className="size-3" /> : <StarIcon className="size-3" />}
                      </button>
                      <button type="button" aria-label="重命名会话" onClick={() => startRename(conversation)} className="rounded p-1 text-neutral-6 hover:bg-neutral-3 hover:text-neutral-9">
                        <Pencil1Icon className="size-3" />
                      </button>
                      <button type="button" aria-label="删除会话" onClick={() => setDeleteTarget(conversation)} className="rounded p-1 text-neutral-6 hover:bg-error-soft hover:text-error">
                        <TrashIcon className="size-3" />
                      </button>
                    </div>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ul>
      </div>

      {managing ? (
        <div className="border-t border-border p-3">
          <div className="flex gap-2">
            <Button
              type="button"
              variant="destructive"
              className="flex-1"
              disabled={selectedIDs.length === 0 || batchDeleting}
              onClick={() => { setBatchError(""); setBatchDeleteOpen(true); }}
            >
              <TrashIcon />删除选中（{selectedIDs.length}）
            </Button>
            <Button type="button" variant="outline" disabled={batchDeleting} onClick={() => { setManaging(false); setSelectedIDs([]); }}>
              取消
            </Button>
          </div>
        </div>
      ) : null}

      <ConfirmDialog
        open={batchDeleteOpen}
        title={`删除选中的 ${selectedIDs.length} 个会话？`}
        description="对话记录会从列表中移除，题库中已生成的草稿不受影响。只能删除你自己创建的会话。"
        confirmLabel={`删除 ${selectedIDs.length} 个会话`}
        isPending={batchDeleting}
        error={batchError || undefined}
        onCancel={() => { if (!batchDeleting) { setBatchDeleteOpen(false); setBatchError(""); } }}
        onConfirm={() => { void confirmBatchDelete(); }}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        title="删除这个会话？"
        description={`「${deleteTarget?.title ?? ""}」的对话记录将从列表中移除。`}
        confirmLabel="删除会话"
        onCancel={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (deleteTarget) onDelete(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}