import { request, requestText } from "./client";
import type {
  AcceptInviteRequest,
  Folder,
  Instance,
  InvitePreview,
  ListSitesQuery,
  LoginRequest,
  PasswordResetConfirm,
  SetupRequest,
  Site,
  SiteList,
  UpdateSiteRequest,
  User,
  Workspace,
} from "./types";

interface Signal {
  signal?: AbortSignal;
}

export async function getSetupStatus({ signal }: Signal = {}): Promise<boolean> {
  const status = await request<{ setupRequired: boolean }>("/setup/status", { signal });
  return status.setupRequired;
}

export async function setup(body: SetupRequest): Promise<User> {
  const result = await request<{ user: User }>("/setup", { method: "POST", body });
  return result.user;
}

export async function login(body: LoginRequest): Promise<User> {
  const result = await request<{ user: User }>("/auth/login", { method: "POST", body });
  return result.user;
}

export async function logout(): Promise<void> {
  await request<void>("/auth/logout", { method: "POST" });
}

export async function getMe({ signal }: Signal = {}): Promise<User> {
  const result = await request<{ user: User }>("/me", { signal });
  return result.user;
}

export async function requestPasswordReset(email: string): Promise<void> {
  await request<unknown>("/auth/password-reset/request", { method: "POST", body: { email } });
}

export async function confirmPasswordReset(body: PasswordResetConfirm): Promise<void> {
  await request<void>("/auth/password-reset/confirm", { method: "POST", body });
}

export async function getInvite(token: string, { signal }: Signal = {}): Promise<InvitePreview> {
  return request<InvitePreview>(`/invites/${encodeURIComponent(token)}`, { signal });
}

export async function acceptInvite(token: string, body: AcceptInviteRequest): Promise<User> {
  const result = await request<{ user: User }>(`/invites/${encodeURIComponent(token)}/accept`, { method: "POST", body });
  return result.user;
}

export async function getInstance({ signal }: Signal = {}): Promise<Instance> {
  return request<Instance>("/instance", { signal });
}

export async function getAgentGuide({ signal }: Signal = {}): Promise<string> {
  return requestText("/docs/agent-guide.md", { signal });
}

export async function getOpenApiDocument({ signal }: Signal = {}): Promise<string> {
  return requestText("/docs/openapi.yaml", { signal });
}

export async function listWorkspaces({ signal }: Signal = {}): Promise<Workspace[]> {
  const result = await request<{ workspaces: Workspace[] }>("/workspaces", { signal });
  return result.workspaces;
}

export async function createWorkspace(name: string): Promise<Workspace> {
  const result = await request<{ workspace: Workspace }>("/workspaces", { method: "POST", body: { name } });
  return result.workspace;
}

export async function listFolders(workspace: string, { signal }: Signal = {}): Promise<Folder[]> {
  const result = await request<{ folders: Folder[] }>(`/workspaces/${encodeURIComponent(workspace)}/folders`, { signal });
  return result.folders;
}

export async function createFolder(workspace: string, name: string): Promise<Folder> {
  const result = await request<{ folder: Folder }>(`/workspaces/${encodeURIComponent(workspace)}/folders`, {
    method: "POST",
    body: { name },
  });
  return result.folder;
}

export async function renameFolder(workspace: string, folderId: string, name: string): Promise<Folder> {
  const result = await request<{ folder: Folder }>(
    `/workspaces/${encodeURIComponent(workspace)}/folders/${encodeURIComponent(folderId)}`,
    { method: "PATCH", body: { name } },
  );
  return result.folder;
}

export async function deleteFolder(workspace: string, folderId: string, force: boolean): Promise<void> {
  await request<void>(`/workspaces/${encodeURIComponent(workspace)}/folders/${encodeURIComponent(folderId)}`, {
    method: "DELETE",
    query: force ? { force: "true" } : undefined,
  });
}

export async function listSites(query: ListSitesQuery, { signal }: Signal = {}): Promise<SiteList> {
  return request<SiteList>("/sites", { query, signal });
}

export async function updateSite(slug: string, body: UpdateSiteRequest): Promise<Site> {
  const result = await request<{ site: Site }>(`/sites/${encodeURIComponent(slug)}`, { method: "PATCH", body });
  return result.site;
}
