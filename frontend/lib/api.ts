import { z } from "zod";

const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

const errorResponseSchema = z.object({
  error: z.object({
    code: z.string().min(1),
    message: z.string().min(1),
    details: z.unknown().optional(),
  }),
  request_id: z.string().min(1),
});

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: unknown;
  readonly requestId: string | null;

  constructor(message: string, status: number, code: string, details?: unknown, requestId?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
    this.requestId = requestId ?? null;
  }
}

function invalidResponse(status: number, error = false) {
  return new ApiError(
    error ? "服务返回了无法识别的错误响应" : "服务返回了无法识别的数据",
    status,
    error ? "INVALID_ERROR_RESPONSE" : "INVALID_RESPONSE",
  );
}

async function fetchApi(path: string, options: RequestInit): Promise<Response> {
  const headers = new Headers(options.headers);
  const isFormData = typeof FormData !== "undefined" && options.body instanceof FormData;
  if (options.body && !isFormData && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  try {
    return await fetch(`${API_URL}${path}`, { ...options, headers, credentials: "include" });
  } catch {
    throw new ApiError("无法连接服务，请检查网络或稍后重试", 0, "NETWORK_ERROR");
  }
}

async function readJsonPayload(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw invalidResponse(response.status, !response.ok);
  }
}

function responseError(response: Response, payload: unknown, validationErrorSchema?: z.ZodType<unknown>): ApiError {
  const parsedError = errorResponseSchema.safeParse(payload);
  if (parsedError.success) {
    const details = validationErrorSchema?.safeParse(parsedError.data.error.details);
    return new ApiError(
      parsedError.data.error.message,
      response.status,
      parsedError.data.error.code,
      details?.success ? details.data : parsedError.data.error.details,
      parsedError.data.request_id,
    );
  }

  const parsedValidation = validationErrorSchema?.safeParse(payload);
  if (parsedValidation?.success) {
    return new ApiError("CSV 文件校验未通过", response.status, "CSV_VALIDATION_ERROR", parsedValidation.data);
  }
  return invalidResponse(response.status, true);
}

export async function apiRequest<T>(
  path: string,
  schema: z.ZodType<T>,
  options: RequestInit = {},
  validationErrorSchema?: z.ZodType<unknown>,
): Promise<T> {
  const response = await fetchApi(path, options);
  const payload = await readJsonPayload(response);
  if (!response.ok) throw responseError(response, payload, validationErrorSchema);

  const parsed = schema.safeParse(payload);
  if (!parsed.success) throw invalidResponse(response.status);
  return parsed.data;
}

export async function apiDownload(path: string): Promise<Blob> {
  const response = await fetchApi(path, {});
  if (!response.ok) {
    const payload = await readJsonPayload(response);
    throw responseError(response, payload);
  }
  return response.blob();
}
