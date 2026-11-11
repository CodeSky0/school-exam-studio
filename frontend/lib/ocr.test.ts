import { afterEach, describe, expect, it, vi } from "vitest";
import { recognizeQuestionImage } from "./ocr";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("OCR API", () => {
  it("使用 multipart 上传且不手动设置 Content-Type", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ text: "已知 $x=2$" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const file = new File(["image"], "question.png", { type: "image/png" });
    await expect(recognizeQuestionImage(file)).resolves.toEqual({ text: "已知 $x=2$" });
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.method).toBe("POST");
    expect(init.body).toBeInstanceOf(FormData);
    expect(new Headers(init.headers).has("Content-Type")).toBe(false);
  });
});
