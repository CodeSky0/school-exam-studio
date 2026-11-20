import { describe, expect, it } from "vitest";
import { aiCloudFileIssue, cloudKindLabel, cloudListSchema, supportsAIConversation, type CloudItem } from "./cloud";

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
});
