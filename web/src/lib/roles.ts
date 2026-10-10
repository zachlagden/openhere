import type { Role, Workspace } from "@/api/types";

const ROLE_LABELS: Record<Role, string> = {
  admin: "Admin",
  write: "Can edit",
  read: "Read only",
};

export function roleLabel(role: Role): string {
  return ROLE_LABELS[role];
}

export function canWriteWholeWorkspace(workspace: Workspace): boolean {
  return !workspace.scopedOnly && (workspace.role === "admin" || workspace.role === "write");
}

export function canWrite(role: Role): boolean {
  return role === "admin" || role === "write";
}
