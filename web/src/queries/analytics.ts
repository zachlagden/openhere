import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/analytics";
import type { Analytics, AnalyticsQuery } from "@/api/types";
import { queryKeys } from "./keys";

export type AnalyticsScope = { kind: "site"; slug: string } | { kind: "workspace"; slug: string } | { kind: "instance" };

function fetchReport(scope: AnalyticsScope, query: AnalyticsQuery, signal: AbortSignal): Promise<Analytics> {
  switch (scope.kind) {
    case "site":
      return api.getSiteAnalytics(scope.slug, query, { signal });
    case "workspace":
      return api.getWorkspaceAnalytics(scope.slug, query, { signal });
    case "instance":
      return api.getInstanceAnalytics(query, { signal });
  }
}

export function useAnalyticsReport(scope: AnalyticsScope, query: AnalyticsQuery, enabled = true): UseQueryResult<Analytics> {
  return useQuery({
    queryKey: [...queryKeys.analytics, scope, query],
    enabled,
    placeholderData: keepPreviousData,
    queryFn: ({ signal }) => fetchReport(scope, query, signal),
  });
}
