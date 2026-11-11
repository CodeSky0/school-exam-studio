import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";
import { ApiError, apiRequest } from "./api";

const responseSchema = z.object({ ok: z.boolean() });

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("API 客户端", () => {
  it("携带 Cookie 并校验成功响应", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiRequest("/api/v1/example", responseSchema)).resolves.toEqual({ ok: true });
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/example",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("解析统一错误结构并保留请求标识", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
      error: {
        code: "VALIDATION_ERROR",
        message: "请求内容未通过校验",
        details: [{ field: "email" }],
      },
      request_id: "req-123",
    }), { status: 422 })));

    const error = await apiRequest("/api/v1/example", responseSchema).catch((value: unknown) => value);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 422,
      code: "VALIDATION_ERROR",
      message: "请求内容未通过校验",
      details: [{ field: "email" }],
      requestId: "req-123",
    });
  });

  it("拒绝结构错误的成功响应", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: "yes" }), { status: 200 })));
    const error = await apiRequest("/api/v1/example", responseSchema).catch((value: unknown) => value);
    expect(error).toMatchObject({ code: "INVALID_RESPONSE", status: 200 });
  });

  it("拒绝不符合约定的错误响应", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ message: "failed" }), { status: 500 })));
    const error = await apiRequest("/api/v1/example", responseSchema).catch((value: unknown) => value);
    expect(error).toMatchObject({ code: "INVALID_ERROR_RESPONSE", status: 500 });
  });

  it("将网络异常转换为可展示错误", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    const error = await apiRequest("/api/v1/example", responseSchema).catch((value: unknown) => value);
    expect(error).toMatchObject({ code: "NETWORK_ERROR", status: 0 });
  });
});
