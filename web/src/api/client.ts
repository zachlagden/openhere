const BASE_PATH = "/api/v1";
const REQUESTED_WITH = "selfhost-dashboard";

export type QueryValue = string | number | boolean | null | undefined;
export type QueryParams = Record<string, QueryValue>;

export interface ErrorBody {
  code?: unknown;
  message?: unknown;
  retry_after?: unknown;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly retryAfter: number | null;

  constructor(status: number, code: string, message: string, retryAfter: number | null) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.retryAfter = retryAfter;
  }
}

export class NetworkError extends Error {
  constructor(cause: unknown) {
    super("Cannot reach the server.", { cause });
    this.name = "NetworkError";
  }
}

export function isApiError(value: unknown): value is ApiError {
  return value instanceof ApiError;
}

export function isNetworkError(value: unknown): value is NetworkError {
  return value instanceof NetworkError;
}

export function isUnauthenticated(value: unknown): boolean {
  return isApiError(value) && value.status === 401;
}

export function buildQuery(params: QueryParams | undefined): string {
  if (!params) {
    return "";
  }
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") {
      continue;
    }
    search.set(key, String(value));
  }
  const text = search.toString();
  return text === "" ? "" : `?${text}`;
}

export interface RequestOptions {
  method?: "GET" | "POST" | "PATCH" | "PUT" | "DELETE";
  query?: QueryParams;
  body?: unknown;
  signal?: AbortSignal;
}

function readRetryAfter(body: ErrorBody | null, response: Response): number | null {
  if (body && typeof body.retry_after === "number" && Number.isFinite(body.retry_after)) {
    return body.retry_after;
  }
  const header = Number(response.headers.get("Retry-After"));
  return Number.isFinite(header) && header > 0 ? header : null;
}

async function readErrorBody(response: Response): Promise<ErrorBody | null> {
  const type = response.headers.get("Content-Type") ?? "";
  if (!type.includes("json")) {
    return null;
  }
  try {
    const parsed: unknown = await response.json();
    return typeof parsed === "object" && parsed !== null ? (parsed as ErrorBody) : null;
  } catch {
    return null;
  }
}

export async function toApiError(response: Response): Promise<ApiError> {
  const body = await readErrorBody(response);
  const code = typeof body?.code === "string" ? body.code : "unknown";
  const message = typeof body?.message === "string" ? body.message : `The server answered ${response.status}.`;
  return new ApiError(response.status, code, message, readRetryAfter(body, response));
}

async function send(path: string, options: RequestOptions): Promise<Response> {
  const method = options.method ?? "GET";
  const headers: Record<string, string> = { Accept: "application/json, text/plain, */*" };
  let body: string | undefined;
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(options.body);
  }
  if (method !== "GET") {
    headers["X-Requested-With"] = REQUESTED_WITH;
  }
  let response: Response;
  try {
    response = await fetch(`${BASE_PATH}${path}${buildQuery(options.query)}`, {
      method,
      headers,
      body,
      signal: options.signal,
      credentials: "same-origin",
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw error;
    }
    throw new NetworkError(error);
  }
  if (!response.ok) {
    throw await toApiError(response);
  }
  return response;
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const response = await send(path, options);
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export async function requestText(path: string, options: RequestOptions = {}): Promise<string> {
  const response = await send(path, options);
  return response.text();
}
