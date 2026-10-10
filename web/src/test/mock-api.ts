import { vi, type Mock } from "vitest";

export interface MockRequest {
  method: string;
  path: string;
  search: URLSearchParams;
  body: unknown;
}

export interface MockResponse {
  status?: number;
  body?: unknown;
  headers?: Record<string, string>;
}

export type Handler = MockResponse | ((request: MockRequest) => MockResponse);

export function mockApi(routes: Record<string, Handler>): Mock {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = new URL(String(input), "http://localhost");
    const method = init?.method ?? "GET";
    const handler = routes[`${method} ${url.pathname}`];
    if (!handler) {
      return new Response(JSON.stringify({ code: "not_found", message: "Not found." }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      });
    }
    const request: MockRequest = {
      method,
      path: url.pathname,
      search: url.searchParams,
      body: typeof init?.body === "string" ? (JSON.parse(init.body) as unknown) : undefined,
    };
    const answer = typeof handler === "function" ? handler(request) : handler;
    const status = answer.status ?? 200;
    if (status === 204) {
      return new Response(null, { status });
    }
    return new Response(JSON.stringify(answer.body ?? {}), {
      status,
      headers: { "Content-Type": "application/json", ...answer.headers },
    });
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

export function apiError(status: number, code: string, message: string, extra: Record<string, unknown> = {}): MockResponse {
  return { status, body: { code, message, ...extra } };
}
