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
};
