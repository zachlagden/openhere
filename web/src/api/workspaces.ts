import { request } from "./client";
import { segment, workspacePath } from "./paths";
import type { AddMemberRequest, CreateInviteRequest, CreatedInvite, Invite, Member, Role, Workspace } from "./types";

interface Signal {
  signal?: AbortSignal;
}

export async function renameWorkspace(workspace: string, name: string): Promise<Workspace> {
  const result = await request<{ workspace: Workspace }>(workspacePath(workspace), { method: "PATCH", body: { name } });
  return result.workspace;
}

export async function deleteWorkspace(workspace: string): Promise<void> {
  await request<void>(workspacePath(workspace), { method: "DELETE" });
}

export async function listMembers(workspace: string, { signal }: Signal = {}): Promise<Member[]> {
  const result = await request<{ members: Member[] }>(`${workspacePath(workspace)}/members`, { signal });
  return result.members;
}

export async function addMember(workspace: string, body: AddMemberRequest): Promise<Member> {
  const result = await request<{ member: Member }>(`${workspacePath(workspace)}/members`, { method: "POST", body });
  return result.member;
}

export async function updateMember(workspace: string, memberId: string, role: Role): Promise<Member> {
  const result = await request<{ member: Member }>(`${workspacePath(workspace)}/members/${segment(memberId)}`, {
    method: "PATCH",
    body: { role },
  });
  return result.member;
}

export async function removeMember(workspace: string, memberId: string): Promise<void> {
  await request<void>(`${workspacePath(workspace)}/members/${segment(memberId)}`, { method: "DELETE" });
}

export async function listInvites({ signal }: Signal = {}): Promise<Invite[]> {
  const result = await request<{ invites: Invite[] }>("/invites", { signal });
  return result.invites;
}

export async function createInvite(body: CreateInviteRequest): Promise<CreatedInvite> {
  return request<CreatedInvite>("/invites", { method: "POST", body });
}

export async function revokeInvite(id: string): Promise<void> {
  await request<void>(`/invites/${segment(id)}`, { method: "DELETE" });
}

export async function resendInvite(id: string, sendEmail: boolean): Promise<CreatedInvite> {
  return request<CreatedInvite>(`/invites/${segment(id)}/resend`, { method: "POST", body: { sendEmail } });
}
