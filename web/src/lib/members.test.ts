import { describe, expect, it } from "vitest";
import { ApiError } from "@/api/client";
import type { Member } from "@/api/types";
import { adminCount, buildAddRequest, describeMemberError, roleOptions, scopeLabel, sortMembers, validateAddMember } from "./members";

function member(overrides: Partial<Member>): Member {
  return { id: "g1", userId: "u1", email: "alex@example.com", role: "admin", scopeType: "workspace", scopeId: null, scopeName: null, createdAt: "2026-03-01T10:00:00Z", ...overrides };
}

describe("members", () => {
  it("labels scopes in plain language", () => {
    expect(scopeLabel(member({}))).toBe("Whole workspace");
    expect(scopeLabel(member({ scopeType: "folder", scopeName: "Reports" }))).toBe("Folder: Reports");
    expect(scopeLabel(member({ scopeType: "site", scopeName: "launch-page" }))).toBe("Site: launch-page");
  });

  it("sorts by person, then by scope", () => {
    const sorted = sortMembers([
      member({ id: "a", email: "sam@example.com", scopeType: "site", scopeName: "z-site" }),
      member({ id: "b", email: "sam@example.com", scopeType: "workspace" }),
      member({ id: "c", email: "alex@example.com", scopeType: "folder", scopeName: "Reports" }),
    ]);
    expect(sorted.map((item) => item.id)).toEqual(["c", "b", "a"]);
  });

  it("offers admin only for whole-workspace grants", () => {
    expect(roleOptions("workspace")).toEqual(["admin", "write", "read"]);
    expect(roleOptions("folder")).toEqual(["write", "read"]);
    expect(roleOptions("site")).toEqual(["write", "read"]);
  });

  it("counts workspace-wide admins", () => {
    expect(adminCount([member({}), member({ id: "g2", role: "write" }), member({ id: "g3", scopeType: "folder", scopeName: "x" })])).toBe(1);
  });

  it("validates the add form", () => {
    expect(validateAddMember({ email: "", role: "read", scopeType: "workspace", scopeId: "" })).toMatch(/email address/);
    expect(validateAddMember({ email: "sam@example.com", role: "read", scopeType: "folder", scopeId: "" })).toBe("Choose a folder.");
    expect(validateAddMember({ email: "sam@example.com", role: "read", scopeType: "site", scopeId: "" })).toBe("Choose a site.");
    expect(validateAddMember({ email: "sam@example.com", role: "admin", scopeType: "folder", scopeId: "f1" })).toMatch(/whole workspace only/);
    expect(validateAddMember({ email: "sam@example.com", role: "write", scopeType: "folder", scopeId: "f1" })).toBeNull();
  });

  it("builds the request without a scope id for workspace grants", () => {
    expect(buildAddRequest({ email: " sam@example.com ", role: "read", scopeType: "workspace", scopeId: "stale" })).toEqual({ email: "sam@example.com", role: "read", scopeType: "workspace" });
    expect(buildAddRequest({ email: "sam@example.com", role: "write", scopeType: "folder", scopeId: "f1" })).toEqual({ email: "sam@example.com", role: "write", scopeType: "folder", scopeId: "f1" });
  });

  it("explains the rule errors", () => {
    expect(describeMemberError(new ApiError(409, "last_admin", "raw", null))).toMatch(/at least one admin/);
    expect(describeMemberError(new ApiError(404, "user_not_found", "raw", null))).toMatch(/Invite them/);
    expect(describeMemberError(new ApiError(400, "validation_failed", "Role must be admin, write or read.", null))).toBe("Role must be admin, write or read.");
  });
});
