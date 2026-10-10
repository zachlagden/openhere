import { request } from "./client";
import { segment } from "./paths";
import type { AdminUser, EmailTestResult, HostingStatus, ResetLink, Settings, UpdateSettingsEnvelope, WildcardCheckResult } from "./types";

interface Signal {
  signal?: AbortSignal;
}

export async function getSettings({ signal }: Signal = {}): Promise<Settings> {
  const result = await request<{ settings: Settings }>("/admin/settings", { signal });
  return result.settings;
}

export async function patchSettings(settings: Settings): Promise<UpdateSettingsEnvelope> {
  return request<UpdateSettingsEnvelope>("/admin/settings", { method: "PATCH", body: { settings } });
}

export async function getHostingStatus({ signal }: Signal = {}): Promise<HostingStatus> {
  return request<HostingStatus>("/hosting/status", { signal });
}

export async function checkWildcard(): Promise<WildcardCheckResult> {
  return request<WildcardCheckResult>("/admin/hosting/check", { method: "POST" });
}

export async function sendTestEmail(): Promise<EmailTestResult> {
  return request<EmailTestResult>("/admin/email/test", { method: "POST" });
}

export async function listUsers({ signal }: Signal = {}): Promise<AdminUser[]> {
  const result = await request<{ users: AdminUser[] }>("/admin/users", { signal });
  return result.users;
}

export async function setUserDisabled(id: string, disabled: boolean): Promise<AdminUser> {
  const result = await request<{ user: AdminUser }>(`/admin/users/${segment(id)}`, { method: "PATCH", body: { disabled } });
  return result.user;
}

export async function createResetLink(id: string): Promise<ResetLink> {
  return request<ResetLink>(`/admin/users/${segment(id)}/password-reset`, { method: "POST" });
}
