import { request } from "./client";
import { sitePath, segment } from "./paths";
import type {
  Access,
  Domain,
  DomainVerifyResult,
  LiveManifest,
  Site,
  UpdateAccessRequest,
  UpdateSiteRequest,
  Version,
  VersionResult,
} from "./types";

interface Signal {
  signal?: AbortSignal;
}

export async function getSite(slug: string, { signal }: Signal = {}): Promise<Site> {
  const result = await request<{ site: Site }>(sitePath(slug), { signal });
  return result.site;
}

export async function patchSite(slug: string, body: UpdateSiteRequest): Promise<Site> {
  const result = await request<{ site: Site }>(sitePath(slug), { method: "PATCH", body });
  return result.site;
}

export async function deleteSite(slug: string): Promise<void> {
  await request<void>(sitePath(slug), { method: "DELETE" });
}

export async function getAccess(slug: string, { signal }: Signal = {}): Promise<Access> {
  const result = await request<{ access: Access }>(`/publish/${segment(slug)}/access`, { signal });
  return result.access;
}

export async function patchAccess(slug: string, body: UpdateAccessRequest): Promise<Access> {
  const result = await request<{ access: Access }>(`/publish/${segment(slug)}/access`, { method: "PATCH", body });
  return result.access;
}

export async function listDomains(slug: string, { signal }: Signal = {}): Promise<Domain[]> {
  const result = await request<{ domains: Domain[] }>(`${sitePath(slug)}/domains`, { signal });
  return result.domains;
}

export async function createDomain(slug: string, domain: string): Promise<Domain> {
  const result = await request<{ domain: Domain }>(`${sitePath(slug)}/domains`, { method: "POST", body: { domain } });
  return result.domain;
}

export async function setPrimaryDomain(slug: string, id: string, isPrimary: boolean): Promise<Domain> {
  const result = await request<{ domain: Domain }>(`${sitePath(slug)}/domains/${segment(id)}`, { method: "PATCH", body: { isPrimary } });
  return result.domain;
}

export async function deleteDomain(slug: string, id: string): Promise<void> {
  await request<void>(`${sitePath(slug)}/domains/${segment(id)}`, { method: "DELETE" });
}

export async function verifyDomain(slug: string, id: string): Promise<DomainVerifyResult> {
  return request<DomainVerifyResult>(`${sitePath(slug)}/domains/${segment(id)}/verify`, { method: "POST" });
}

export async function listVersions(slug: string, { signal }: Signal = {}): Promise<Version[]> {
  const result = await request<{ versions: Version[] }>(`/publish/${segment(slug)}/versions`, { signal });
  return result.versions;
}

export async function restoreVersion(slug: string, versionId: string): Promise<VersionResult> {
  return request<VersionResult>(`/publish/${segment(slug)}/versions/${segment(versionId)}/restore`, { method: "POST" });
}

export async function deleteVersion(slug: string, versionId: string): Promise<void> {
  await request<void>(`/publish/${segment(slug)}/versions/${segment(versionId)}`, { method: "DELETE" });
}

export async function listLiveFiles(slug: string, { signal }: Signal = {}): Promise<LiveManifest> {
  return request<LiveManifest>(`${sitePath(slug)}/files`, { signal });
}
