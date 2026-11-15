"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  adminQueryKeys,
  getSchool,
  getSchoolAISettings,
  updateSchoolAISettings,
  updateSchoolName,
  type School,
  type SchoolAISettings,
} from "@/lib/admin";
import { ApiError } from "@/lib/api";
import { schoolNameSchema } from "@/lib/schemas";
import { AppShell } from "@/components/app-shell";
import { FormSkeleton } from "@/components/auth-form";
import { PageHeader } from "@/components/page-header";
import { RoleAccess } from "@/components/role-access";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { StatusMessage } from "@/components/ui/status-message";

function SchoolForm({ school }: { school: School }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(school.name);
  const [localError, setLocalError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const mutation = useMutation({
    mutationFn: updateSchoolName,
    onSuccess: (updated) => {
      queryClient.setQueryData(adminQueryKeys.school, updated);
      setName(updated.name);
      setMessage("学校名称已保存");
    },
  });

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const result = schoolNameSchema.safeParse(name);
    if (!result.success) {
      setLocalError(result.error.issues[0]?.message ?? "请输入有效学校名称");
      setMessage(null);
      return;
    }
    setLocalError(null);
    setMessage(null);
    mutation.mutate(result.data);
  }

  const mutationMessage = mutation.error instanceof ApiError ? mutation.error.message : "学校名称保存失败，请稍后重试";

  return (
    <form onSubmit={submit} noValidate className="max-w-4xl border-t border-border pt-7">
      <div className="grid gap-7">
        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label htmlFor="school-name" className="pt-2.5">学校名称</Label>
          <div className="grid gap-2">
            <Input id="school-name" value={name} aria-invalid={Boolean(localError)} aria-describedby="school-name-message" className="h-11 bg-[#fffdf8]" onChange={(event) => { setName(event.target.value); setMessage(null); }} />
            {localError ? <p id="school-name-message" role="alert" className="text-copy-13 text-error">{localError}</p> : <p id="school-name-message" className="text-label-12 text-neutral-7">2 至 200 个字符，保存时仅提交学校名称。</p>}
          </div>
        </div>
        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label htmlFor="school-code" className="pt-2.5">学校代码（只读）</Label>
          <div className="grid gap-2">
            <Input id="school-code" value={school.school_code} readOnly aria-readonly className="h-11 border-accent/20 bg-accent-soft font-mono tracking-[0.16em]" />
            <p className="text-label-12 leading-5 text-neutral-7">学校代码用于教师注册，不能在此修改。</p>
          </div>
        </div>
        <dl className="grid gap-4 border-y border-border py-5 text-label-12 sm:ml-40 sm:grid-cols-2 sm:pl-6">
          <div><dt className="text-neutral-7">学校 ID</dt><dd className="mt-1.5 break-all font-mono text-neutral-8">{school.id}</dd></div>
          <div><dt className="text-neutral-7">最近更新</dt><dd className="mt-1.5 break-all text-neutral-8"><time dateTime={school.updated_at}>{school.updated_at}</time></dd></div>
        </dl>
        <div className="grid gap-3 sm:ml-40 sm:pl-6">
          {mutation.isError ? <StatusMessage message={mutationMessage} tone="error" /> : null}
          <StatusMessage message={message} tone="success" />
          <div><Button type="submit" size="lg" disabled={mutation.isPending}>{mutation.isPending ? "正在保存…" : "保存学校名称"}</Button></div>
        </div>
      </div>
    </form>
  );
}

const vendorPresets = [
  {
    id: "openai",
    label: "OpenAI",
    baseURL: "https://api.openai.com/v1",
    models: ["gpt-4o-mini", "gpt-4o", "gpt-4.1-mini"],
  },
  {
    id: "deepseek",
    label: "DeepSeek",
    baseURL: "https://api.deepseek.com/v1",
    models: ["deepseek-chat", "deepseek-reasoner"],
  },
  {
    id: "qwen",
    label: "通义千问 / 阿里云百炼",
    baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
    models: ["qwen-plus", "qwen-max", "qwen-turbo"],
  },
  {
    id: "zhipu",
    label: "智谱 AI",
    baseURL: "https://open.bigmodel.cn/api/paas/v4",
    models: ["glm-4-plus", "glm-4-flash", "glm-4-air"],
  },
  {
    id: "moonshot",
    label: "Moonshot / Kimi",
    baseURL: "https://api.moonshot.cn/v1",
    models: ["moonshot-v1-8k", "moonshot-v1-32k"],
  },
  {
    id: "custom",
    label: "自定义 / 其他 OpenAI 兼容接口",
    baseURL: "",
    models: [],
  },
] as const;

interface AIFormState {
  provider: string;
  baseURL: string;
  apiKey: string;
  models: string[];
  defaultModel: string;
  enabled: boolean;
  clearAPIKey: boolean;
}

function toFormState(settings: SchoolAISettings): AIFormState {
  return {
    provider: settings.provider || "custom",
    baseURL: settings.base_url,
    apiKey: "",
    models: settings.models,
    defaultModel: settings.default_model,
    enabled: settings.enabled || (!settings.configured && settings.source === "env"),
    clearAPIKey: false,
  };
}

function AISettingsForm({ settings }: { settings: SchoolAISettings }) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<AIFormState>(() => toFormState(settings));
  const [customModel, setCustomModel] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const preset = vendorPresets.find((item) => item.id === form.provider);
  const candidates = Array.from(new Set([...(preset?.models ?? []), ...form.models]));
  const mutation = useMutation({
    mutationFn: updateSchoolAISettings,
    onSuccess: (updated) => {
      queryClient.setQueryData(adminQueryKeys.aiSettings, updated);
      setForm(toFormState(updated));
      setMessage("全校 AI 模型配置已保存，教师端将在下次调用时生效。");
      setLocalError(null);
    },
  });

  function chooseVendor(provider: string) {
    const nextPreset = vendorPresets.find((item) => item.id === provider);
    setForm((prev) => {
      const models = Array.from(new Set([...prev.models, ...(nextPreset?.models ?? [])]));
      return {
        ...prev,
        provider,
        baseURL: nextPreset?.baseURL || prev.baseURL,
        models,
        defaultModel: models.includes(prev.defaultModel) ? prev.defaultModel : (models[0] ?? ""),
      };
    });
    setMessage(null);
  }

  function toggleModel(model: string) {
    setForm((prev) => {
      const enabled = prev.models.includes(model);
      const models = enabled ? prev.models.filter((item) => item !== model) : [...prev.models, model];
      return {
        ...prev,
        models,
        defaultModel: models.includes(prev.defaultModel) ? prev.defaultModel : (models[0] ?? ""),
      };
    });
    setMessage(null);
  }

  function addCustomModel() {
    const model = customModel.trim();
    if (!model) return;
    if (!form.models.includes(model)) {
      setForm((prev) => ({ ...prev, models: [...prev.models, model], defaultModel: prev.defaultModel || model }));
    }
    setCustomModel("");
    setMessage(null);
  }

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (form.enabled) {
      if (!form.baseURL.trim()) {
        setLocalError("启用全校 AI 前必须填写接口地址");
        return;
      }
      if (!form.apiKey.trim() && !settings.has_api_key && !form.clearAPIKey) {
        setLocalError("启用全校 AI 前必须填写 API Key");
        return;
      }
      if (!form.apiKey.trim() && form.clearAPIKey) {
        setLocalError("清除已保存密钥后，请填写新的 API Key 或关闭启用");
        return;
      }
      if (form.models.length === 0) {
        setLocalError("启用全校 AI 前至少启用一个模型");
        return;
      }
    }
    if (form.baseURL.trim() && !/^https?:\/\//.test(form.baseURL.trim())) {
      setLocalError("接口地址必须以 http:// 或 https:// 开头");
      return;
    }
    setLocalError(null);
    setMessage(null);
    mutation.mutate({
      provider: form.provider,
      base_url: form.baseURL.trim(),
      api_key: form.apiKey.trim() || undefined,
      models: form.models,
      default_model: form.defaultModel,
      enabled: form.enabled,
      clear_api_key: form.clearAPIKey,
    });
  }

  const mutationMessage = mutation.error instanceof ApiError ? mutation.error.message : "AI 模型配置保存失败，请稍后重试";
  const sourceLabel = settings.source === "env"
    ? "当前继承服务器环境变量"
    : settings.enabled
      ? "全校教师共用"
      : "当前使用本地引擎";

  return (
    <form onSubmit={submit} noValidate className="mt-10 max-w-4xl border-t border-border pt-8">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-caption-10 font-medium uppercase tracking-[0.18em] text-neutral-6">AI 模型服务</p>
          <h2 className="mt-2 font-display text-title-20 text-neutral-10">全校共用模型</h2>
          <p className="mt-2 max-w-2xl text-copy-13 leading-6 text-neutral-7">
            管理员统一维护厂商、接口地址与 API Key，并选择教师可用的模型。API Key 加密保存，不会返回浏览器；未配置或停用时自动退回本地确定性引擎。
          </p>
        </div>
        <span className={form.enabled ? "rounded-full border border-success/25 bg-success-soft px-3 py-1 text-label-12 text-success" : "rounded-full border border-border-strong bg-neutral-2 px-3 py-1 text-label-12 text-neutral-7"}>
          {sourceLabel}
        </span>
      </div>

      <div className="mt-7 grid gap-7">
        {settings.source === "env" && !settings.configured ? (
          <StatusMessage message="当前继承服务器环境变量。保存任意配置后将改为学校独立配置，服务器默认模型将不再对本校生效。" tone="info" />
        ) : null}
        <label className="flex items-start gap-3 rounded-lg border border-border bg-neutral-2/50 px-4 py-3">
          <input
            type="checkbox"
            className="mt-1 size-4 accent-accent"
            checked={form.enabled}
            onChange={(event) => { setForm((prev) => ({ ...prev, enabled: event.target.checked })); setMessage(null); }}
          />
          <span>
            <span className="block text-copy-13 font-medium text-neutral-9">允许全校教师使用共享模型</span>
            <span className="mt-1 block text-label-12 leading-5 text-neutral-7">关闭后所有 AI 请求使用本地引擎，不会调用外部模型。</span>
          </span>
        </label>

        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label htmlFor="ai-provider" className="pt-2.5">模型厂商</Label>
          <div className="grid gap-2">
            <Select id="ai-provider" value={form.provider} onChange={(event) => chooseVendor(event.target.value)}>
              {vendorPresets.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </Select>
            <p className="text-label-12 text-neutral-7">选择厂商会填入推荐接口地址，并合并推荐模型。</p>
          </div>
        </div>

        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label htmlFor="ai-base-url" className="pt-2.5">接口地址</Label>
          <div className="grid gap-2">
            <Input id="ai-base-url" value={form.baseURL} placeholder="https://api.example.com/v1" className="h-11 bg-[#fffdf8] font-mono text-copy-13" onChange={(event) => { setForm((prev) => ({ ...prev, baseURL: event.target.value })); setMessage(null); }} />
            <p className="text-label-12 leading-5 text-neutral-7">需兼容 OpenAI Chat Completions，例如 https://api.deepseek.com/v1。</p>
          </div>
        </div>

        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label htmlFor="ai-api-key" className="pt-2.5">API Key</Label>
          <div className="grid gap-2">
            <Input
              id="ai-api-key"
              type="password"
              autoComplete="new-password"
              value={form.apiKey}
              placeholder={settings.has_api_key ? `已保存 ${settings.api_key_hint || "密钥"}，留空保持不变` : "输入厂商 API Key"}
              className="h-11 bg-[#fffdf8] font-mono text-copy-13"
              onChange={(event) => { setForm((prev) => ({ ...prev, apiKey: event.target.value })); setMessage(null); }}
            />
            <div className="flex flex-wrap items-center gap-3 text-label-12 text-neutral-7">
              <span>密钥使用服务器密钥加密存储，仅在调用模型时解密。</span>
              {settings.has_api_key ? (
                <label className="flex items-center gap-2 text-error">
                  <input type="checkbox" className="size-3.5 accent-error" checked={form.clearAPIKey} onChange={(event) => setForm((prev) => ({ ...prev, clearAPIKey: event.target.checked }))} />
                  清除已保存密钥
                </label>
              ) : null}
            </div>
          </div>
        </div>

        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label className="pt-2.5">启用模型</Label>
          <div className="grid gap-3">
            <div className="flex flex-wrap gap-2">
              {candidates.length === 0 ? <p className="text-copy-13 text-neutral-7">选择厂商后会显示推荐模型，也可以手动添加。</p> : null}
              {candidates.map((model) => {
                const active = form.models.includes(model);
                return (
                  <button
                    key={model}
                    type="button"
                    aria-pressed={active}
                    onClick={() => toggleModel(model)}
                    className={active ? "rounded-full border border-accent/40 bg-accent-soft px-3 py-1.5 font-mono text-label-12 text-accent-strong" : "rounded-full border border-border-strong bg-neutral-1 px-3 py-1.5 font-mono text-label-12 text-neutral-7 hover:border-accent/30 hover:text-neutral-9"}
                  >
                    {model}
                  </button>
                );
              })}
            </div>
            <div className="flex flex-wrap gap-2">
              <Input
                value={customModel}
                placeholder="添加其他模型名称"
                className="h-10 max-w-xs bg-[#fffdf8] font-mono text-copy-13"
                onChange={(event) => setCustomModel(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    addCustomModel();
                  }
                }}
              />
              <Button type="button" variant="outline" size="sm" onClick={addCustomModel} disabled={!customModel.trim()}>添加模型</Button>
            </div>
            <p className="text-label-12 leading-5 text-neutral-7">教师只能选择这里启用的模型；至少启用一个模型后才会调用大模型。</p>
          </div>
        </div>

        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label htmlFor="ai-default-model" className="pt-2.5">默认模型</Label>
          <div className="grid gap-2">
            <Select id="ai-default-model" value={form.defaultModel} onChange={(event) => { setForm((prev) => ({ ...prev, defaultModel: event.target.value })); setMessage(null); }}>
              {form.models.length === 0 ? <option value="">尚未启用模型</option> : null}
              {form.models.map((model) => <option key={model} value={model}>{model}</option>)}
            </Select>
            <p className="text-label-12 text-neutral-7">未指定模型的 AI 请求将使用默认模型。</p>
          </div>
        </div>

        <div className="grid gap-3 sm:ml-40 sm:pl-6">
          {localError ? <StatusMessage message={localError} tone="error" /> : null}
          {mutation.isError ? <StatusMessage message={mutationMessage} tone="error" /> : null}
          <StatusMessage message={message} tone="success" />
          {settings.warning ? <StatusMessage message={settings.warning} tone="warning" /> : null}
          <div className="flex flex-wrap items-center gap-3">
            <Button type="submit" size="lg" disabled={mutation.isPending}>{mutation.isPending ? "正在保存…" : "保存 AI 模型配置"}</Button>
            {settings.updated_at ? <span className="text-label-12 text-neutral-7">最近更新：<time dateTime={settings.updated_at}>{settings.updated_at}</time></span> : null}
          </div>
        </div>
      </div>
    </form>
  );
}

function AISettingsSection() {
  const query = useQuery({ queryKey: adminQueryKeys.aiSettings, queryFn: getSchoolAISettings, staleTime: 0 });
  const queryMessage = query.error instanceof ApiError ? query.error.message : "AI 模型配置读取失败，请稍后重试";
  if (query.isPending) {
    return <div className="mt-10 max-w-3xl border-t border-border py-8"><FormSkeleton /></div>;
  }
  if (query.error) {
    return (
      <div className="mt-10 max-w-3xl border-t border-border py-6">
        <StatusMessage message={queryMessage} tone="error" />
        <Button variant="outline" size="sm" className="mt-3" onClick={() => { void query.refetch(); }}>重新读取</Button>
      </div>
    );
  }
  return query.data ? <AISettingsForm settings={query.data} /> : null;
}

function SchoolSettingsContent() {
  const query = useQuery({ queryKey: adminQueryKeys.school, queryFn: getSchool, staleTime: 0 });
  const queryMessage = query.error instanceof ApiError ? query.error.message : "学校资料读取失败，请稍后重试";
  return (
    <div className="mx-auto max-w-6xl">
      <PageHeader eyebrow="管理员工作区 / 学校资料" title="学校设置" description="维护学校名称与全校教师共用的 AI 模型服务。" className="pb-7" />
      {query.isPending ? <div className="max-w-3xl border-t border-border py-8"><FormSkeleton /></div> : null}
      {query.error ? <div className="max-w-3xl border-t border-border py-6"><StatusMessage message={queryMessage} tone="error" /><Button variant="outline" size="sm" className="mt-3" onClick={() => { void query.refetch(); }}>重新读取</Button></div> : null}
      {query.data ? <SchoolForm school={query.data} /> : null}
      <AISettingsSection />
    </div>
  );
}

export function SchoolSettings() {
  return <RoleAccess role="admin">{(user) => <AppShell user={user}><SchoolSettingsContent /></AppShell>}</RoleAccess>;
}
