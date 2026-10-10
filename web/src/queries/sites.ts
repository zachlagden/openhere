import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQueryClient,
  type InfiniteData,
  type UseInfiniteQueryResult,
  type UseMutationResult,
} from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import type { Site, SiteList } from "@/api/types";
import { queryKeys, type SitesKeyParams } from "./keys";

export const SITES_PAGE_SIZE = 50;

export function useSites(params: SitesKeyParams): UseInfiniteQueryResult<InfiniteData<SiteList, string | undefined>> {
  return useInfiniteQuery({
    queryKey: queryKeys.siteList(params),
    initialPageParam: undefined as string | undefined,
    placeholderData: keepPreviousData,
    queryFn: ({ pageParam, signal }) =>
      api.listSites(
        {
          workspace: params.workspace,
          folder: params.folder,
          q: params.q,
          limit: SITES_PAGE_SIZE,
          cursor: pageParam,
        },
        { signal },
      ),
    getNextPageParam: (page) => page.nextCursor ?? undefined,
  });
}

export function useMoveSite(): UseMutationResult<Site, Error, { slug: string; folderId: string | null }> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ slug, folderId }) => api.updateSite(slug, { folderId }),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.sites }),
  });
}
