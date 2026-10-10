import { describe, expect, it } from "vitest";
import { ApiError } from "@/api/client";
import type { AdminUser } from "@/api/types";
import { canDisable, canResetPassword, describeUserError, sortUsers } from "./admin-users";

const user = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: "u2",
  email: "sam@example.com",
  displayName: "",
  isRoot: false,
  disabled: false,
  disabledAt: null,
  createdAt: "2026-03-01T10:00:00Z",
  ...overrides,
});

describe("admin user rules", () => {
  it("never offers to disable yourself or a disabled account", () => {
    expect(canDisable(user(), "u1")).toBe(true);
    expect(canDisable(user({ id: "u1" }), "u1")).toBe(false);
    expect(canDisable(user({ disabled: true }), "u1")).toBe(false);
  });

  it("offers reset links only for enabled accounts", () => {
    expect(canResetPassword(user())).toBe(true);
    expect(canResetPassword(user({ disabled: true }))).toBe(false);
  });

  it("lists root users first, then by email", () => {
    const sorted = sortUsers([user({ id: "b", email: "zed@example.com" }), user({ id: "a", email: "amy@example.com" }), user({ id: "r", email: "root@example.com", isRoot: true })]);
    expect(sorted.map((item) => item.id)).toEqual(["r", "a", "b"]);
  });

  it("explains the refusals in plain language", () => {
    expect(describeUserError(new ApiError(409, "cannot_disable_self", "x", null))).toMatch(/your own account/);
    expect(describeUserError(new ApiError(409, "last_root", "x", null))).toMatch(/last root user/);
    expect(describeUserError(new ApiError(409, "user_disabled", "x", null))).toMatch(/Enable it/);
  });
});
