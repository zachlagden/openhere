import { afterEach, describe, expect, it, vi } from "vitest";
import { apiError, mockApi } from "@/test/mock-api";
import { ApiError, buildQuery, NetworkError, request } from "./client";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("buildQuery", () => {
  it("skips empty values", () => {
    expect(buildQuery({ q: "a b", folder: "", cursor: undefined, limit: 50 })).toBe("?q=a+b&limit=50");
    expect(buildQuery({})).toBe("");
  });
});

describe("request", () => {
  it("sends X-Requested-With on state-changing calls only", async () => {
    const fetchMock = mockApi({ "GET /api/v1/me": { body: {} }, "POST /api/v1/auth/logout": { status: 204 } });
    await request("/me");
    await request("/auth/logout", { method: "POST" });
    const [, getInit] = fetchMock.mock.calls[0] as [string, RequestInit];
    const [, postInit] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect((getInit.headers as Record<string, string>)["X-Requested-With"]).toBeUndefined();
    expect((postInit.headers as Record<string, string>)["X-Requested-With"]).toBeTruthy();
  });

  it("turns the JSON error shape into an ApiError", async () => {
    mockApi({ "GET /api/v1/me": apiError(429, "rate_limited", "Too many attempts.", { retry_after: 30 }) });
    await expect(request("/me")).rejects.toMatchObject({ status: 429, code: "rate_limited", retryAfter: 30 });
  });

  it("handles an error without a JSON body", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("bad gateway", { status: 502 })));
    const error = await request("/me").catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 502, code: "unknown" });
  });

  it("wraps a failed fetch in a NetworkError", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("Failed to fetch"))));
    await expect(request("/me")).rejects.toBeInstanceOf(NetworkError);
  });
});
