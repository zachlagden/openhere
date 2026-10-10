import { request } from "./client";
import { sitePath, workspacePath } from "./paths";
import type { Analytics, AnalyticsQuery } from "./types";

interface Signal {
  signal?: AbortSignal;
}

export async function getSiteAnalytics(slug: string, query: AnalyticsQuery, { signal }: Signal = {}): Promise<Analytics> {
  return request<Analytics>(`${sitePath(slug)}/analytics`, { query, signal });
}

export async function getWorkspaceAnalytics(workspace: string, query: AnalyticsQuery, { signal }: Signal = {}): Promise<Analytics> {
  return request<Analytics>(`${workspacePath(workspace)}/analytics`, { query, signal });
}

export async function getInstanceAnalytics(query: AnalyticsQuery, { signal }: Signal = {}): Promise<Analytics> {
  return request<Analytics>("/admin/analytics", { query, signal });
}
