import type { Domain, Folder, Instance, Site, User, Workspace } from "@/api/types";

export const alex: User = {
  id: "u1",
  email: "alex@example.com",
  displayName: "",
  isRoot: true,
  createdAt: "2026-03-01T10:00:00Z",
};

export const personal: Workspace = {
  id: "w1",
  slug: "personal",
  name: "Personal",
  kind: "personal",
  role: "admin",
  scopedOnly: false,
  createdAt: "2026-03-01T10:00:00Z",
  updatedAt: "2026-03-01T10:00:00Z",
};

export const reportsFolder: Folder = {
  id: "f1",
  workspaceId: "w1",
  name: "Reports",
  role: "admin",
  createdAt: "2026-03-01T10:00:00Z",
  updatedAt: "2026-03-01T10:00:00Z",
};

export function makeSite(overrides: Partial<Site> = {}): Site {
  return {
    id: "s1",
    workspaceId: "w1",
    workspaceSlug: "personal",
    folderId: null,
    slug: "launch-page",
    displayName: "Launch page",
    description: "",
    createdBy: "u1",
    spaMode: false,
    analyticsEnabled: true,
    accessMode: "public",
    currentVersionId: "v1",
    url: "https://sites.example.com/s/launch-page/",
    hosting: {
      reachable: true,
      url: "https://sites.example.com/s/launch-page/",
      modes: {
        subdomain: { state: "off", reason: "switched_off" },
        path: { state: "on", reason: null },
        customDomains: { state: "on", reason: null },
      },
      notices: [{ code: "subdomain_off", message: "The subdomain address for this site is switched off." }],
    },
    role: "admin",
    createdAt: "2026-03-01T10:00:00Z",
    updatedAt: "2026-03-01T10:00:00Z",
    ...overrides,
  };
}

export const instanceInfo: Instance = {
  publicUrl: "https://sites.example.com",
  version: "1.0.0",
  emailAvailable: false,
  canCreateWorkspaces: true,
  hostingModes: { subdomain: false, path: true, customDomains: true },
  maxDomainsPerSite: 3,
  whoCanInvite: "root",
  agentCredentialsDir: ".selfhost",
  agentCredentialsPath: "~/.selfhost/credentials",
};

export function makeDomain(overrides: Partial<Domain> = {}): Domain {
  return {
    id: "d1",
    domain: "docs.example.org",
    status: "pending_verification",
    challenge: { recordType: "TXT", name: "_selfhost-challenge.docs.example.org", value: "ab12cd34ef56" },
    target: { recordType: "CNAME", value: "sites.example.com", note: "Use an ALIAS or flattened CNAME record on an apex domain." },
    isPrimary: false,
    verifiedAt: null,
    lastCheckedAt: null,
    lastCheckResult: null,
    createdAt: "2026-03-01T10:00:00Z",
    ...overrides,
  };
}
