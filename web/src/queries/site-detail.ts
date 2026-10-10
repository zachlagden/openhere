import { useMutation, useQuery, useQueryClient, type QueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/sites";
import type { Access, Domain, DomainVerifyResult, LiveManifest, Site, UpdateAccessRequest, UpdateSiteRequest, Version, VersionResult } from "@/api/types";
import { queryKeys } from "./keys";

export function useSite(slug: string): UseQueryResult<Site> {
  return useQuery({
    queryKey: queryKeys.site(slug),
    queryFn: ({ signal }) => api.getSite(slug, { signal }),
  });
}

function refreshSites(client: QueryClient): Promise<void> {
  return client.invalidateQueries({ queryKey: queryKeys.sites });
}

export function useUpdateSite(slug: string): UseMutationResult<Site, Error, UpdateSiteRequest> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateSiteRequest) => api.patchSite(slug, body),
    onSuccess: async (site) => {
      client.setQueryData(queryKeys.site(slug), site);
      await refreshSites(client);
    },
  });
}

export function useDeleteSite(slug: string): UseMutationResult<void, Error, void> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: () => api.deleteSite(slug),
    onSuccess: async () => {
      client.removeQueries({ queryKey: queryKeys.site(slug) });
      await Promise.all([refreshSites(client), client.invalidateQueries({ queryKey: queryKeys.allDomains })]);
    },
  });
}

export function useAccess(slug: string): UseQueryResult<Access> {
  return useQuery({
    queryKey: queryKeys.siteAccess(slug),
    queryFn: ({ signal }) => api.getAccess(slug, { signal }),
  });
}

export function useUpdateAccess(slug: string): UseMutationResult<Access, Error, UpdateAccessRequest> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateAccessRequest) => api.patchAccess(slug, body),
    onSuccess: async (access) => {
      client.setQueryData(queryKeys.siteAccess(slug), access);
      await refreshSites(client);
    },
  });
}

export function useDomains(slug: string): UseQueryResult<Domain[]> {
  return useQuery({
    queryKey: queryKeys.siteDomains(slug),
    queryFn: ({ signal }) => api.listDomains(slug, { signal }),
  });
}

function refreshDomains(client: QueryClient, slug: string): Promise<void[]> {
  return Promise.all([
    client.invalidateQueries({ queryKey: queryKeys.siteDomains(slug) }),
    client.invalidateQueries({ queryKey: queryKeys.allDomains }),
    refreshSites(client),
  ]);
}

export function useCreateDomain(slug: string): UseMutationResult<Domain, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (domain: string) => api.createDomain(slug, domain),
    onSuccess: () => refreshDomains(client, slug),
  });
}

export function useVerifyDomain(slug: string): UseMutationResult<DomainVerifyResult, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.verifyDomain(slug, id),
    onSuccess: () => refreshDomains(client, slug),
  });
}

export function useSetPrimaryDomain(slug: string): UseMutationResult<Domain, Error, { id: string; isPrimary: boolean }> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ id, isPrimary }) => api.setPrimaryDomain(slug, id, isPrimary),
    onSuccess: () => refreshDomains(client, slug),
  });
}

export function useDeleteDomain(slug: string): UseMutationResult<void, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteDomain(slug, id),
    onSuccess: () => refreshDomains(client, slug),
  });
}

export function useVersions(slug: string): UseQueryResult<Version[]> {
  return useQuery({
    queryKey: queryKeys.siteVersions(slug),
    queryFn: ({ signal }) => api.listVersions(slug, { signal }),
  });
}

export function useLiveFiles(slug: string): UseQueryResult<LiveManifest> {
  return useQuery({
    queryKey: queryKeys.siteFiles(slug),
    queryFn: ({ signal }) => api.listLiveFiles(slug, { signal }),
  });
}

export function useRestoreVersion(slug: string): UseMutationResult<VersionResult, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (versionId: string) => api.restoreVersion(slug, versionId),
    onSuccess: () => refreshSites(client),
  });
}

export function useDeleteVersion(slug: string): UseMutationResult<void, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (versionId: string) => api.deleteVersion(slug, versionId),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.siteVersions(slug) }),
  });
}
