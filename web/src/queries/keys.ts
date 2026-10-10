export interface SitesKeyParams {
  workspace: string;
  folder: string;
  q: string;
}

export const queryKeys = {
  me: ["me"] as const,
  setupStatus: ["setup-status"] as const,
  instance: ["instance"] as const,
  agentGuide: ["docs", "agent-guide"] as const,
  openApi: ["docs", "openapi"] as const,
  workspaces: ["workspaces"] as const,
  folders: (workspace: string) => ["workspaces", workspace, "folders"] as const,
  sites: ["sites"] as const,
  siteList: (params: SitesKeyParams) => ["sites", "list", params] as const,
  site: (slug: string) => ["sites", "detail", slug] as const,
  siteAccess: (slug: string) => ["sites", "detail", slug, "access"] as const,
  siteDomains: (slug: string) => ["sites", "detail", slug, "domains"] as const,
  siteVersions: (slug: string) => ["sites", "detail", slug, "versions"] as const,
  siteFiles: (slug: string) => ["sites", "detail", slug, "files"] as const,
  allDomains: ["domains-overview"] as const,
  members: (workspace: string) => ["workspaces", workspace, "members"] as const,
  invites: ["invites"] as const,
  apiKeys: ["api-keys"] as const,
  analytics: ["analytics"] as const,
  adminSettings: ["admin", "settings"] as const,
  adminUsers: ["admin", "users"] as const,
  hostingStatus: ["hosting-status"] as const,
};
