import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/account";
import type { ApiKey, CreatedApiKey } from "@/api/types";
import { queryKeys } from "./keys";

export function useApiKeys(): UseQueryResult<ApiKey[]> {
  return useQuery({
    queryKey: queryKeys.apiKeys,
    queryFn: ({ signal }) => api.listApiKeys({ signal }),
  });
}

export function useCreateApiKey(): UseMutationResult<CreatedApiKey, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.createApiKey,
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.apiKeys }),
  });
}

export function useRevokeApiKey(): UseMutationResult<void, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.revokeApiKey,
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.apiKeys }),
  });
}
