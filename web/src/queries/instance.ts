import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import type { Instance } from "@/api/types";
import { queryKeys } from "./keys";

export function useInstance(): UseQueryResult<Instance> {
  return useQuery({
    queryKey: queryKeys.instance,
    staleTime: 60_000,
    queryFn: ({ signal }) => api.getInstance({ signal }),
  });
}

export function useAgentGuide(): UseQueryResult<string> {
  return useQuery({
    queryKey: queryKeys.agentGuide,
    staleTime: 10 * 60_000,
    queryFn: ({ signal }) => api.getAgentGuide({ signal }),
  });
}
