import { describe, expect, it } from "vitest";
import { ApiError, NetworkError } from "./client";
import { describeError, formatWait } from "./errors";

describe("formatWait", () => {
  it("rounds up and picks a unit", () => {
    expect(formatWait(1)).toBe("1 second");
    expect(formatWait(30)).toBe("30 seconds");
    expect(formatWait(600)).toBe("10 minutes");
    expect(formatWait(7200)).toBe("2 hours");
  });
});

describe("describeError", () => {
  it("explains a rate limit with the wait", () => {
    const result = describeError(new ApiError(429, "rate_limited", "Too many attempts. Try again later.", 45));
    expect(result).toEqual({ kind: "rate_limited", message: "Too many attempts. Try again in 45 seconds.", retryAfter: 45 });
  });

  it("handles a rate limit without a wait", () => {
    expect(describeError(new ApiError(429, "rate_limited", "x", null)).message).toBe("Too many attempts. Try again later.");
  });

  it("explains a busy server with the wait", () => {
    const result = describeError(new ApiError(503, "server_busy", "The server is busy. Try again in a moment.", 5));
    expect(result).toEqual({ kind: "busy", message: "The server is busy. Try again in 5 seconds.", retryAfter: 5 });
    expect(describeError(new ApiError(503, "server_busy", "x", null)).message).toBe("The server is busy. Try again in a moment.");
  });

  it("maps network failures", () => {
    expect(describeError(new NetworkError(new Error("down"))).kind).toBe("network");
  });

  it("uses the server message for unmapped codes", () => {
    const result = describeError(new ApiError(409, "folder_name_taken", "A folder with that name exists.", null));
    expect(result.message).toBe("A folder with that name exists.");
    expect(result.kind).toBe("other");
  });

  it("classifies statuses", () => {
    expect(describeError(new ApiError(401, "unauthenticated", "x", null)).kind).toBe("unauthenticated");
    expect(describeError(new ApiError(403, "forbidden", "No.", null)).kind).toBe("forbidden");
    expect(describeError(new ApiError(404, "not_found", "x", null)).kind).toBe("not_found");
  });

  it("falls back for unknown values", () => {
    expect(describeError("boom").message).toBe("Something went wrong. Try again.");
  });
});
