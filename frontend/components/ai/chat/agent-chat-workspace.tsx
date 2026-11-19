"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Cross2Icon,
  HamburgerMenuIcon,
  LayersIcon,
  LightningBoltIcon,
  MagicWandIcon,
  PlusIcon,
} from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { aiQueryKeys, getAIStatus } from "@/lib/ai";
import {
  aiChatQueryKeys,
  batchDeleteConversations,
  createConversation,
  deleteConversation,
  listConversations,
  listMessages,
  listSkills,
  listTools,
  MAX_ATTACHMENTS,
  streamMessage,
  updateConversation,
  uploadAttachment,
  type AttachmentRef,
  type ChatMessage,
  type Conversation,
  type LiveTurn,
  type Skill,
  type TimelineEntry,
} from "@/lib/ai-chat";
import { paperQueryKeys } from "@/lib/papers";
import { questionQueryKeys } from "@/lib/questions";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ConversationSidebar } from "./conversation-sidebar";
import { MessageList } from "./message-list";
import { Composer } from "./composer";
import { SkillsPanel } from "./skills-panel";
import type { UploadTask } from "./attachment-tray";

function initialPrompt() {
  if (typeof window === "undefined") return "";
  return new URLSearchParams(window.location.search).get("prompt") ?? "";
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "操作失败，请稍后重试";
}

function isAbortError(error: unknown) {
  return error instanceof DOMException && error.name === "AbortError";
}

const IMAGE_TYPES = ["image/png", "image/jpeg", "image/webp", "image/gif"];
const IMAGE_EXTS = [".png", ".jpg", ".jpeg", ".webp", ".gif"];
const DOCUMENT_EXTS = [".pdf", ".docx", ".txt", ".md", ".csv"];

function uploadValidation(file: File) {
  const lower = file.name.toLowerCase();
  // 与服务端分类保持一致：只有 png/jpeg/webp/gif 按图片处理，SVG 等其他图片类型会在服务端被拒绝。
  const isImage = IMAGE_TYPES.includes(file.type) || (file.type === "" && IMAGE_EXTS.some((ext) => lower.endsWith(ext)));
  if (isImage && file.size > 10 * 1024 * 1024) return "图片不能超过 10 MiB";
  if (!isImage && file.size > 20 * 1024 * 1024) return "文档不能超过 20 MiB";
  const allowed = isImage || DOCUMENT_EXTS.some((ext) => lower.endsWith(ext));
  if (!allowed) return "仅支持 PNG/JPEG/WebP/GIF 图片，以及 PDF、DOCX、TXT、Markdown、CSV";
  return "";
}

export function AgentChatWorkspace({ user }: { user: CurrentUser }) {
  const scope: "teacher" | "admin" = user.role === "admin" ? "admin" : "teacher";
  const queryClient = useQueryClient();
  const abortRef = useRef<AbortController | null>(null);
  // 附件上传期间切换/清空会话时递增，用于丢弃已失效的上传回调，避免附件串到别的会话。
  const uploadEpochRef = useRef(0);
  const [activeID, setActiveID] = useState("");
  const [draft, setDraft] = useState(initialPrompt);
  const [attachments, setAttachments] = useState<AttachmentRef[]>([]);
  const [uploads, setUploads] = useState<UploadTask[]>([]);
  const [model, setModel] = useState("");
  const [liveTurn, setLiveTurn] = useState<LiveTurn | null>(null);
  const [actionError, setActionError] = useState("");
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [panelOpen, setPanelOpen] = useState(false);
  const [creating, setCreating] = useState(false);

  const conversationsQuery = useQuery({
    queryKey: aiChatQueryKeys.conversations,
    queryFn: () => listConversations(40),
  });
  const conversations = conversationsQuery.data?.items ?? [];
  // 管理员能看到全校会话，默认只落在自己创建的会话上，避免一进来就是别人的只读对话。
  const activeConversation = activeID
    ? conversations.find((item) => item.id === activeID)
    : conversations.find((item) => item.creator_id === user.id);
  const effectiveActiveID = activeConversation?.id ?? "";
  const messagesQuery = useQuery({
    queryKey: aiChatQueryKeys.messages(effectiveActiveID),
    queryFn: () => listMessages(effectiveActiveID),
    enabled: Boolean(effectiveActiveID),
  });
  const statusQuery = useQuery({ queryKey: aiQueryKeys.status, queryFn: getAIStatus });
  const skillsQuery = useQuery({ queryKey: aiChatQueryKeys.skills, queryFn: listSkills, staleTime: 5 * 60_000 });
  const toolsQuery = useQuery({ queryKey: aiChatQueryKeys.tools, queryFn: listTools, staleTime: 5 * 60_000 });

  const messages = messagesQuery.data?.items ?? [];
  const skills = skillsQuery.data?.items ?? [];
  const tools = toolsQuery.data?.items ?? [];
  const effectiveModel = model || statusQuery.data?.default_model || "";
  const readOnly = Boolean(activeConversation && activeConversation.creator_id !== user.id);
  const isSending = liveTurn?.status === "running" || creating;

  useEffect(() => () => abortRef.current?.abort(), []);

  const createMutation = useMutation({
    mutationFn: () => createConversation(),
    onSuccess: (conversation) => {
      uploadEpochRef.current += 1;
      setActiveID(conversation.id);
      setDraft("");
      setAttachments([]);
      setUploads([]);
      setActionError("");
      void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.conversations });
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  const updateMutation = useMutation({
    mutationFn: (variables: { id: string; title?: string; pinned?: boolean }) => updateConversation(variables.id, {
      title: variables.title,
      pinned: variables.pinned,
    }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.conversations });
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteConversation(id),
    onSuccess: (_data, id) => {
      queryClient.setQueryData<{ items: Conversation[] }>(aiChatQueryKeys.conversations, (current) => current
        ? { ...current, items: current.items.filter((item) => item.id !== id) }
        : current);
      if (id === effectiveActiveID) {
        uploadEpochRef.current += 1;
        setActiveID("");
        setDraft("");
        setAttachments([]);
        setUploads([]);
      }
      void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.conversations });
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  const batchDeleteMutation = useMutation({
    mutationFn: (ids: string[]) => batchDeleteConversations(ids),
    onSuccess: (result) => {
      const removed = new Set(result.deleted);
      queryClient.setQueryData<{ items: Conversation[] }>(aiChatQueryKeys.conversations, (current) => current
        ? { ...current, items: current.items.filter((item) => !removed.has(item.id)) }
        : current);
      if (removed.has(effectiveActiveID)) {
        uploadEpochRef.current += 1;
        setActiveID("");
        setDraft("");
        setAttachments([]);
        setUploads([]);
        setLiveTurn(null);
      }
      if (result.failed.length > 0) {
        setActionError(`已删除 ${result.deleted.length} 个会话，${result.failed.length} 个未能删除`);
      }
      void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.conversations });
    },
    onError: (error) => setActionError(errorMessage(error)),
  });

  function stopStreaming() {
    abortRef.current?.abort();
    abortRef.current = null;
    setLiveTurn(null);
    void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.messages(effectiveActiveID) });
  }

  function selectConversation(id: string) {
    if (id === effectiveActiveID) return;
    abortRef.current?.abort();
    abortRef.current = null;
    uploadEpochRef.current += 1;
    setActiveID(id);
    setDraft("");
    setAttachments([]);
    setUploads([]);
    setLiveTurn(null);
    setActionError("");
    setSidebarOpen(false);
  }

  function appendMessageToCache(conversationID: string, message: ChatMessage) {
    queryClient.setQueryData<{ items: ChatMessage[] }>(aiChatQueryKeys.messages(conversationID), (current) => {
      const items = current?.items ?? [];
      if (items.some((item) => item.id === message.id)) return current;
      return { items: [...items, message] };
    });
  }

  async function handleSend() {
    const content = draft.trim();
    if (isSending || readOnly) return;
    if (!content && attachments.length === 0) return;
    if (uploads.some((item) => item.status === "uploading")) {
      setActionError("请等待附件解析完成后再发送");
      return;
    }
    if (uploads.some((item) => item.status === "failed")) {
      setActionError("有附件上传失败，请移除失败项后重试");
      return;
    }
    setActionError("");
    let conversationID = effectiveActiveID;
    if (!conversationID) {
      setCreating(true);
      try {
        const conversation = await createConversation();
        conversationID = conversation.id;
        setActiveID(conversation.id);
        void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.conversations });
      } catch (error) {
        setCreating(false);
        setActionError(errorMessage(error));
        return;
      }
      setCreating(false);
    }
    const sentAttachments = attachments;
    setLiveTurn({
      content: content || "请解析这些附件",
      startedAt: new Date().toISOString(),
      attachments: sentAttachments,
      toolCalls: [],
      timeline: [],
      assistantContent: "",
      reasoning: "",
      status: "running",
    });
    setDraft("");
    setAttachments([]);
    setUploads([]);
    uploadEpochRef.current += 1;
    const controller = new AbortController();
    abortRef.current = controller;
    try {
      await streamMessage(conversationID, {
        content,
        attachment_ids: sentAttachments.map((item) => item.id),
        model: effectiveModel,
      }, (event) => {
        switch (event.type) {
          case "user_message":
            if (event.message) {
              appendMessageToCache(conversationID, event.message);
              // 用户消息已落库并进入消息列表，隐藏本地实时气泡避免重复显示。
              setLiveTurn((current) => current ? { ...current, userMessagePersisted: true } : current);
            }
            break;
          case "assistant_delta": {
            const delta = event.content ?? "";
            if (delta) {
              setLiveTurn((current) => {
                if (!current) return current;
                // 轮次之间的分段换行不单独渲染，避免回复开头出现空白。
                if (!delta.trim() && !current.assistantContent) return current;
                return { ...current, assistantContent: current.assistantContent + delta };
              });
            }
            break;
          }
          case "reasoning_delta":
            if (event.reasoning) {
              setLiveTurn((current) => current
                ? { ...current, reasoning: current.reasoning + event.reasoning }
                : current);
            }
            break;
          case "tool_started": {
            const call = event.tool_call;
            if (!call) break;
            setLiveTurn((current) => {
              if (!current) return current;
              const entryTime = call.started_at ?? new Date().toISOString();
              // 工具开始前，把当前轮的推理与叙述按发生顺序归档进 timeline。
              const pending: TimelineEntry[] = [];
              if (current.reasoning.trim()) {
                pending.push({ type: "thought", content: current.reasoning.trim(), created_at: entryTime });
              }
              if (current.assistantContent.trim()) {
                pending.push({ type: "thought", content: current.assistantContent.trim(), created_at: entryTime });
              }
              let timeline = pending.length > 0 ? [...current.timeline, ...pending] : current.timeline;
              if (!timeline.some((item) => item.type === "tool" && item.tool_call_id === call.id)) {
                timeline = [...timeline, {
                  type: "tool" as const,
                  content: "",
                  tool_call_id: call.id,
                  created_at: entryTime,
                }];
              }
              const toolCalls = current.toolCalls.some((item) => item.id === call.id)
                ? current.toolCalls.map((item) => (item.id === call.id ? call : item))
                : [...current.toolCalls, call];
              return { ...current, toolCalls, timeline, assistantContent: "", reasoning: "" };
            });
            break;
          }
          case "tool_finished": {
            const call = event.tool_call;
            if (!call) break;
            setLiveTurn((current) => {
              if (!current) return current;
              const toolCalls = current.toolCalls.some((item) => item.id === call.id)
                ? current.toolCalls.map((item) => (item.id === call.id ? call : item))
                : [...current.toolCalls, call];
              return { ...current, toolCalls };
            });
            break;
          }
          case "assistant_message":
            if (event.message) {
              // 先写入缓存再替换实时气泡，保证最终消息衔接无损。
              appendMessageToCache(conversationID, event.message);
              setLiveTurn(null);
            }
            break;
          case "error": {
            const message = event.error ?? "AI 处理失败，请稍后重试";
            setActionError(message);
            setLiveTurn((current) => current ? { ...current, status: "failed", error: message } : current);
            break;
          }
          case "done":
            setLiveTurn(null);
            void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.conversations });
            void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.messages(conversationID) });
            void queryClient.invalidateQueries({ queryKey: questionQueryKeys.all });
            void queryClient.invalidateQueries({ queryKey: paperQueryKeys.all });
            break;
        }
      }, controller.signal);
    } catch (error) {
      if (isAbortError(error)) {
        setLiveTurn(null);
        void queryClient.invalidateQueries({ queryKey: aiChatQueryKeys.messages(conversationID) });
        return;
      }
      setLiveTurn((current) => current
        ? { ...current, status: "failed", error: errorMessage(error) }
        : current);
    } finally {
      if (abortRef.current === controller) abortRef.current = null;
    }
  }

  const handleUploadFiles = useCallback((files: File[]) => {
    const available = MAX_ATTACHMENTS - attachments.length - uploads.filter((item) => item.status === "uploading").length;
    if (available <= 0) {
      setActionError(`每轮最多上传 ${MAX_ATTACHMENTS} 个附件`);
      return;
    }
    const selected = files.slice(0, available);
    if (files.length > selected.length) {
      setActionError(`每轮最多上传 ${MAX_ATTACHMENTS} 个附件，已忽略多余文件`);
    }
    const tasks: UploadTask[] = [];
    const valid: File[] = [];
    for (const file of selected) {
      const invalid = uploadValidation(file);
      const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
      if (invalid) {
        tasks.push({ id, name: file.name, status: "failed", error: invalid });
      } else {
        valid.push(file);
        tasks.push({ id, name: file.name, status: "uploading" });
      }
    }
    if (tasks.length > 0) setUploads((current) => [...current, ...tasks]);
    const epoch = uploadEpochRef.current;
    valid.forEach((file, index) => {
      const task = tasks.filter((item) => item.status === "uploading")[index];
      uploadAttachment(file, effectiveActiveID || undefined).then((attachment) => {
        // 上传期间已切换或清空会话，丢弃结果，避免把附件加到错误的会话输入框。
        if (uploadEpochRef.current !== epoch) return;
        if (attachment.status === "failed") {
          setUploads((current) => current.map((item) => item.id === task.id
            ? { ...item, status: "failed", error: attachment.error ?? "解析失败" }
            : item));
          return;
        }
        setUploads((current) => current.filter((item) => item.id !== task.id));
        setAttachments((current) => current.some((item) => item.id === attachment.id) ? current : [...current, attachment]);
      }).catch((error: unknown) => {
        if (uploadEpochRef.current !== epoch) return;
        setUploads((current) => current.map((item) => item.id === task.id
          ? { ...item, status: "failed", error: errorMessage(error) }
          : item));
      });
    });
  }, [effectiveActiveID, attachments.length, uploads]);

  function useSkill(skill: Skill) {
    setDraft(skill.prompt);
    setActionError("");
    setPanelOpen(false);
  }

  function togglePin(conversation: Conversation) {
    updateMutation.mutate({ id: conversation.id, pinned: !conversation.pinned });
  }

  const error = [
    actionError,
    messagesQuery.error ? errorMessage(messagesQuery.error) : "",
    conversationsQuery.error ? errorMessage(conversationsQuery.error) : "",
  ].filter(Boolean).join("；");

  const headerTitle = activeConversation?.title ?? "新的 AI 对话";
  const provider = statusQuery.data?.provider ?? "local";
  const providerLabel = provider === "llm" ? "大模型" : provider === "local_fallback" ? "本地降级" : "本地引擎";
  const models = useMemo(() => statusQuery.data?.models ?? [], [statusQuery.data?.models]);

  return (
    <div className="motion-rise flex h-[calc(100dvh-6.5rem)] min-h-[30rem] flex-col overflow-hidden rounded-xl border border-border bg-paper shadow-sm lg:h-[calc(100dvh-9rem)]">
      <div className="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border bg-neutral-1 px-3 sm:px-4">
        <div className="flex min-w-0 items-center gap-2">
          <Button type="button" size="icon" variant="ghost" className="lg:hidden" aria-label="打开会话历史" onClick={() => setSidebarOpen(true)}>
            <HamburgerMenuIcon />
          </Button>
          <span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-accent/25 bg-accent-soft text-accent-strong">
            <MagicWandIcon className="size-4" />
          </span>
          <div className="min-w-0">
            <p className="truncate text-copy-13 font-medium text-neutral-9">{headerTitle}</p>
            <p className="truncate text-caption-10 text-neutral-6">{readOnly ? "只读" : scope === "admin" ? "管理员" : user.subject}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <span className={cn("hidden items-center gap-1 rounded-full border px-2.5 py-1 text-caption-10 font-medium sm:inline-flex", provider === "llm" ? "border-accent/30 bg-accent-soft text-accent-strong" : "border-info/25 bg-info-soft text-info")}>
            <LightningBoltIcon className="size-3" />{providerLabel}
          </span>
          <Button type="button" size="icon" variant="ghost" aria-label="新建对话" onClick={() => createMutation.mutate()} disabled={createMutation.isPending}>
            <PlusIcon />
          </Button>
          <Button type="button" size="icon" variant="ghost" aria-label="技能与工具" onClick={() => setPanelOpen(true)}>
            <LayersIcon />
          </Button>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)] lg:grid-cols-[16rem_minmax(0,1fr)]">
        {sidebarOpen ? (
          <button type="button" aria-label="关闭会话历史" className="fixed inset-0 z-30 bg-neutral-10/25 backdrop-blur-sm lg:hidden" onClick={() => setSidebarOpen(false)} />
        ) : null}
        <aside
          className={cn(
            "fixed inset-y-0 left-0 z-40 w-72 border-r border-border bg-neutral-1 transition-transform duration-200 ease-out lg:static lg:z-auto lg:flex lg:w-auto lg:translate-x-0",
            sidebarOpen ? "flex translate-x-0" : "flex -translate-x-full",
          )}
          aria-label="会话历史"
        >
          <div className="flex w-full flex-col">
            <div className="flex h-12 shrink-0 items-center justify-between border-b border-border px-3 lg:hidden">
              <span className="text-copy-13 font-medium text-neutral-9">对话历史</span>
              <Button type="button" size="icon" variant="ghost" aria-label="关闭会话历史" onClick={() => setSidebarOpen(false)}><Cross2Icon /></Button>
            </div>
            <div className="min-h-0 flex-1">
              <ConversationSidebar
                conversations={conversations}
                activeID={effectiveActiveID}
                isLoading={conversationsQuery.isPending}
                error={conversationsQuery.error ? errorMessage(conversationsQuery.error) : ""}
                onSelect={selectConversation}
                onCreate={() => createMutation.mutate()}
                onRename={(id, title) => updateMutation.mutate({ id, title })}
                onTogglePin={togglePin}
                onDelete={(id) => deleteMutation.mutate(id)}
                onDeleteMany={async (ids) => { await batchDeleteMutation.mutateAsync(ids); }}
              />
            </div>
          </div>
        </aside>

        <section className="flex min-h-0 min-w-0 flex-col bg-paper" aria-label="AI 对话">
          <MessageList
            messages={messages}
            liveTurn={liveTurn}
            error={error}
            skills={readOnly ? [] : skills}
            scope={scope}
            onUseSkill={useSkill}
          />
          <Composer
            value={draft}
            onChange={setDraft}
            onSubmit={() => { void handleSend(); }}
            onCancel={stopStreaming}
            attachments={attachments}
            uploads={uploads}
            isSending={isSending}
            models={models}
            model={effectiveModel}
            onModelChange={setModel}
            onRemoveAttachment={(id) => setAttachments((current) => current.filter((item) => item.id !== id))}
            onDismissUpload={(id) => setUploads((current) => current.filter((item) => item.id !== id))}
            onUploadFiles={handleUploadFiles}
            skills={readOnly ? [] : skills}
            onUseSkill={useSkill}
            readOnly={readOnly}
          />
        </section>

        {panelOpen ? (
          <button type="button" aria-label="关闭技能与工具" className="fixed inset-0 z-30 bg-neutral-10/25 backdrop-blur-sm" onClick={() => setPanelOpen(false)} />
        ) : null}
        <aside
          className={cn(
            "fixed inset-y-0 right-0 z-40 w-80 max-w-[85vw] border-l border-border bg-neutral-1 shadow-[0_4px_24px_rgba(20,19,18,0.06)] transition-transform duration-200 ease-out",
            panelOpen ? "block translate-x-0" : "hidden translate-x-full",
          )}
          aria-label="技能与工具"
        >
          <div className="flex h-full flex-col">
            <div className="flex h-12 shrink-0 items-center justify-between border-b border-border px-3">
              <span className="text-copy-13 font-medium text-neutral-9">技能与工具</span>
              <Button type="button" size="icon" variant="ghost" aria-label="关闭技能与工具" onClick={() => setPanelOpen(false)}><Cross2Icon /></Button>
            </div>
            <div className="min-h-0 flex-1">
              <SkillsPanel
                status={statusQuery.data}
                isStatusPending={statusQuery.isPending}
                statusError={statusQuery.isError}
                skills={skills}
                tools={tools}
                onUseSkill={useSkill}
              />
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}
