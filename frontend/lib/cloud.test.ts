import { describe, expect, it } from "vitest";
import {
  aiCloudFileIssue,
  cloudIndexLabel,
  cloudKindLabel,
  cloudListSchema,
  cloudSearchSchema,
  supportsAIConversation,
  type CloudItem,
} from "./cloud";

function item(overrides: Partial<CloudItem> = {}): CloudItem {
  return {
    id: "f1",
    parent_id: "",
    name: "资料.pdf",
    is_folder: false,
    mime: "application/pdf",
    kind: "pdf",
    size_bytes: 1024,
    creator_id: "u1",
    index_status: "ready",
    index_error: "",
    index_provider: "local-hash-v1",
    chunk_count: 3,
    indexed_at: "2026-11-20T00:00:00Z",
    created_at: "2026-11-20T00:00:00Z",
    updated_at: "2026-11-20T00:00:00Z",
    ...overrides,
  };
}

describe("cloud helpers", () => {
  it("parses list responses with defaults", () => {
    const parsed = cloudListSchema.parse({ items: [{ id: "1", name: "a", created_at: "now", updated_at: "now" }] });
    expect(parsed.parent_id).toBe("");
    expect(parsed.breadcrumbs).toEqual([]);
    expect(parsed.items[0].is_folder).toBe(false);
  });

  it("only marks AI-parseable formats as selectable", () => {
    expect(supportsAIConversation(item())).toBe(true);
    expect(supportsAIConversation(item({ name: "课件.pptx", kind: "pptx" }))).toBe(false);
    expect(supportsAIConversation(item({ is_folder: true }))).toBe(false);
    expect(aiCloudFileIssue(item({ name: "课件.pptx", kind: "pptx" }))).toBe("AI 暂不支持此格式");
    expect(aiCloudFileIssue(item({ size_bytes: 21 * 1024 * 1024 }))).toBe("文档超过 20 MiB");
    expect(aiCloudFileIssue(item({ kind: "image", name: "scan.png", size_bytes: 11 * 1024 * 1024 }))).toBe("图片超过 10 MiB");
  });

  it("labels common file kinds", () => {
    expect(cloudKindLabel(item())).toBe("PDF");
    expect(cloudKindLabel(item({ kind: "docx", name: "a.docx" }))).toBe("Word");
    expect(cloudKindLabel(item({ kind: "pptx", name: "a.pptx" }))).toBe("PPT");
    expect(cloudKindLabel(item({ is_folder: true }))).toBe("文件夹");
  });

  it("labels vector index status", () => {
    expect(cloudIndexLabel(item())).toBe("已向量化 · 3 段");
    expect(cloudIndexLabel(item({ index_status: "processing" }))).toBe("向量化中…");
    expect(cloudIndexLabel(item({ index_status: "failed" }))).toBe("向量化失败");
    expect(cloudIndexLabel(item({ index_status: "unsupported" }))).toBe("不支持向量化");
    expect(cloudIndexLabel(item({ is_folder: true }))).toBe("");
  });

  it("parses search responses with defaults", () => {
    const parsed = cloudSearchSchema.parse({ query: "方程", hits: [{ item_id: "f1", name: "教案.pdf" }] });
    expect(parsed.index_available).toBe(false);
    expect(parsed.hits[0].score).toBe(0);
    expect(parsed.hits[0].excerpt).toBe("");
  });
});
