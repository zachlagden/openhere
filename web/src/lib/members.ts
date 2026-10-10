import { describeError } from "@/api/errors";
import { isApiError } from "@/api/client";
import type { AddMemberRequest, Member, Role } from "@/api/types";

export type ScopeType = Member["scopeType"];

const SCOPE_ORDER: Record<ScopeType, number> = { workspace: 0, folder: 1, site: 2 };

export function scopeLabel(member: Pick<Member, "scopeType" | "scopeName">): string {
  switch (member.scopeType) {
    case "workspace":
      return "Whole workspace";
    case "folder":
      return `Folder: ${member.scopeName ?? "unknown"}`;
    case "site":
      return `Site: ${member.scopeName ?? "unknown"}`;
  }
}

export function sortMembers(members: Member[]): Member[] {
  return [...members].sort(
    (left, right) =>
      left.email.localeCompare(right.email) ||
      SCOPE_ORDER[left.scopeType] - SCOPE_ORDER[right.scopeType] ||
      (left.scopeName ?? "").localeCompare(right.scopeName ?? ""),
  );
}

export function roleOptions(scope: ScopeType): Role[] {
  return scope === "workspace" ? ["admin", "write", "read"] : ["write", "read"];
}

export function adminCount(members: Member[]): number {
  return members.filter((member) => member.scopeType === "workspace" && member.role === "admin").length;
}

export interface AddMemberForm {
  email: string;
  role: Role;
  scopeType: ScopeType;
  scopeId: string;
}

export function validateAddMember(form: AddMemberForm): string | null {
  if (form.email.trim() === "") {
    return "Enter the email address of an existing account.";
  }
  if (form.scopeType !== "workspace" && form.scopeId === "") {
    return form.scopeType === "folder" ? "Choose a folder." : "Choose a site.";
  }
  if (form.scopeType !== "workspace" && form.role === "admin") {
    return "Admin applies to the whole workspace only.";
  }
  return null;
}

export function buildAddRequest(form: AddMemberForm): AddMemberRequest {
  const base: AddMemberRequest = { email: form.email.trim(), role: form.role, scopeType: form.scopeType };
  return form.scopeType === "workspace" ? base : { ...base, scopeId: form.scopeId };
}

const MEMBER_MESSAGES: Record<string, string> = {
  user_not_found: "No account uses that email address. Invite them to create one, then add them here.",
  member_exists: "That person already has this access. Change their role in the list instead.",
  personal_workspace: "Personal workspaces cannot have other members.",
  last_admin: "A workspace needs at least one admin. Make someone else an admin first.",
};

export function describeMemberError(error: unknown): string {
  if (isApiError(error) && error.code in MEMBER_MESSAGES) {
    return MEMBER_MESSAGES[error.code] ?? error.message;
  }
  return describeError(error).message;
}
