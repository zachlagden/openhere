import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import * as sitesApi from "@/api/sites";
import type { Site } from "@/api/types";
import { toRows, type DomainRow } from "@/lib/domains-overview";
import { queryKeys } from "./keys";

const PAGE_SIZE = 200;
const MAX_PAGES = 10;

export interface DomainsOverview {
  rows: DomainRow[];
  unreadable: number;
  truncated: boolean;
}

async function loadSites(signal: AbortSignal): Promise<{ sites: Site[]; truncated: boolean }> {
  const sites: Site[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < MAX_PAGES; page += 1) {
    const result = await api.listSites({ limit: PAGE_SIZE, cursor }, { signal });
    sites.push(...result.sites);
    if (result.nextCursor === null) {
      return { sites, truncated: false };
    }
    cursor = result.nextCursor;
  }
  return { sites, truncated: true };
}

export function useDomainsOverview(): UseQueryResult<DomainsOverview> {
  return useQuery({
    queryKey: queryKeys.allDomains,
    queryFn: async ({ signal }) => {
      const { sites, truncated } = await loadSites(signal);
      const settled = await Promise.allSettled(sites.map((site) => sitesApi.listDomains(site.slug, { signal })));
      const entries: { site: Site; domains: Awaited<ReturnType<typeof sitesApi.listDomains>> }[] = [];
      let unreadable = 0;
      settled.forEach((result, index) => {
        const site = sites[index];
        if (site === undefined) {
          return;
        }
        if (result.status === "fulfilled") {
          entries.push({ site, domains: result.value });
        } else {
          unreadable += 1;
        }
      });
      return { rows: toRows(entries), unreadable, truncated };
    },
  });
}
