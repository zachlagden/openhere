import type { Instance, Workspace } from "@/api/types";

export function isWorkspaceAdmin(workspace: Workspace): boolean {
  return !workspace.scopedOnly && workspace.role === "admin";
}

export function canManageMembers(workspace: Workspace): boolean {
  return workspace.kind === "shared" && isWorkspaceAdmin(workspace);
}

export function canInvite(workspace: Workspace, isRoot: boolean, whoCanInvite: Instance["whoCanInvite"] | undefined): boolean {
  if (workspace.kind !== "shared") {
    return false;
  }
  if (isRoot) {
    return true;
  }
  return whoCanInvite === "workspaceAdmins" && isWorkspaceAdmin(workspace);
}

export function canSeeWorkspaceAnalytics(workspace: Workspace): boolean {
  return !workspace.scopedOnly;
}

export function canRename(workspace: Workspace): boolean {
  return isWorkspaceAdmin(workspace);
}
