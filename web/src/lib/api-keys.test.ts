import { describe, expect, it } from "vitest";
import { ApiError } from "@/api/client";
import type { ApiKey } from "@/api/types";
import { activeKeyCount, describeKeyError, keyStatus, validateLabel } from "./api-keys";

const key = (overrides: Partial<ApiKey> = {}): ApiKey => ({ id: "k1", label: "ci runner", prefix: "sk_ab12", createdAt: "2026-03-01T10:00:00Z", lastUsedAt: null, revokedAt: null, ...overrides });

describe("api keys", () => {
  it("counts only active keys", () => {
    expect(activeKeyCount([key(), key({ id: "k2", revokedAt: "2026-03-02T10:00:00Z" })])).toBe(1);
  });

  it("labels the status", () => {
    expect(keyStatus(key()).label).toBe("Active");
    expect(keyStatus(key({ revokedAt: "2026-03-02T10:00:00Z" })).label).toBe("Revoked");
  });

  it("validates labels", () => {
    expect(validateLabel("  ")).toMatch(/label/);
    expect(validateLabel("a".repeat(65))).toMatch(/at most 64/);
    expect(validateLabel("ci runner")).toBeNull();
  });

  it("explains the key limit", () => {
    expect(describeKeyError(new ApiError(409, "api_key_limit_reached", "x", null))).toMatch(/50 active keys/);
  });
});
