import { describe, expect, it } from "vitest";
import type { Workspace } from "@/api/types";
import { canInvite, canManageMembers, canSeeWorkspaceAnalytics } from "./workspace-access";

const shared: Workspace = {
  id: "w2",
  slug: "acme",
  name: "Acme Ltd",
  kind: "shared",
  role: "admin",
  scopedOnly: false,
  createdAt: "2026-03-01T10:00:00Z",
  updatedAt: "2026-03-01T10:00:00Z",
};

describe("workspace access", () => {
  it("lets only workspace-wide admins of shared workspaces manage members", () => {
    expect(canManageMembers(shared)).toBe(true);
    expect(canManageMembers({ ...shared, role: "write" })).toBe(false);
    expect(canManageMembers({ ...shared, kind: "personal" })).toBe(false);
    expect(canManageMembers({ ...shared, scopedOnly: true, role: "read" })).toBe(false);
  });

  it("follows the invite policy", () => {
    expect(canInvite(shared, true, "root")).toBe(true);
    expect(canInvite(shared, false, "root")).toBe(false);
    expect(canInvite(shared, false, "workspaceAdmins")).toBe(true);
    expect(canInvite({ ...shared, role: "write" }, false, "workspaceAdmins")).toBe(false);
    expect(canInvite({ ...shared, kind: "personal" }, true, "root")).toBe(false);
    expect(canInvite(shared, false, undefined)).toBe(false);
  });

  it("denies workspace analytics to scoped-only users", () => {
    expect(canSeeWorkspaceAnalytics(shared)).toBe(true);
    expect(canSeeWorkspaceAnalytics({ ...shared, scopedOnly: true, role: "read" })).toBe(false);
  });
});
