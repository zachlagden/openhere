import type { Analytics, Domain, Folder, Instance, Member, Settings, Site, User, Workspace } from "@/api/types";

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

export function makeReport(overrides: Partial<Analytics> = {}): Analytics {
  return {
    from: "2026-03-01",
    to: "2026-03-03",
    interval: "day",
    totals: { views: 30, uniques: 18, botViews: 4 },
    series: [
      { start: "2026-03-01T00:00:00Z", views: 5, uniques: 3 },
      { start: "2026-03-02T00:00:00Z", views: 20, uniques: 12 },
      { start: "2026-03-03T00:00:00Z", views: 5, uniques: 3 },
    ],
    paths: [
      { value: "/", views: 20, uniques: 12 },
      { value: "/pricing", views: 10, uniques: 6 },
    ],
    referrers: [{ value: "news.example.com", views: 8, uniques: 5 }],
    countries: [],
    cities: [],
    browsers: [{ value: "Firefox", views: 30, uniques: 18 }],
    operatingSystems: [{ value: "Linux", views: 30, uniques: 18 }],
    devices: [{ value: "desktop", views: 30, uniques: 18 }, { value: "bot", views: 4, uniques: 4 }],
    statuses: [{ value: "2xx", views: 30, uniques: 18 }],
    locationAvailable: false,
    generatedAt: "2026-03-03T12:00:00Z",
    ...overrides,
  };
}

export const acme: Workspace = {
  id: "w2",
  slug: "acme",
  name: "Acme Ltd",
  kind: "shared",
  role: "admin",
  scopedOnly: false,
  createdAt: "2026-03-01T10:00:00Z",
  updatedAt: "2026-03-01T10:00:00Z",
};

export function makeMember(overrides: Partial<Member> = {}): Member {
  return {
    id: "g1",
    userId: "u1",
    email: "alex@example.com",
    role: "admin",
    scopeType: "workspace",
    scopeId: null,
    scopeName: null,
    createdAt: "2026-03-01T10:00:00Z",
    ...overrides,
  };
}

export function makeSettings(overrides: Partial<Settings> = {}): Settings {
  return {
    "retention.keepVersions": 10,
    "limits.maxFilesPerVersion": 2500,
    "limits.maxVersionBytes": 1_073_741_824,
    "limits.maxFileBytes": 268_435_456,
    "hosting.baseDomain": "example.com",
    "hosting.subdomainEnabled": false,
    "hosting.pathEnabled": true,
    "hosting.customDomainsEnabled": true,
    "hosting.maxDomainsPerSite": 10,
    "hosting.wildcardStatus": "unchecked",
    "hosting.wildcardCheckedAt": null,
    "hosting.wildcardDetail": "",
    "hosting.wildcardFailures": 0,
    "hosting.subdomainDisabledReason": null,
    "sites.defaultAccessMode": "public",
    "invites.whoCanInvite": "root",
    "workspaces.whoCanCreate": "any",
    "email.provider": "none",
    "email.fromAddress": "",
    "email.fromName": "",
    "email.smtp.host": "",
    "email.smtp.port": 587,
    "email.smtp.security": "starttls",
    "email.smtp.username": "",
    "email.smtp.passwordSet": false,
    "email.resend.apiKeySet": false,
    "tls.dnsProvider": "none",
    "tls.cloudflareApiTokenSet": false,
    "analytics.enabledByDefault": true,
    "analytics.retentionDays": 395,
    "analytics.rawDays": 31,
    "analytics.maxmindAccountId": "",
    "analytics.maxmindLicenseKeySet": false,
    "analytics.geoStatus": "unconfigured",
    "analytics.geoUpdatedAt": null,
    "analytics.geoCheckedAt": null,
    "analytics.geoDetail": "",
    ...overrides,
  } as Settings;
}
