import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";
import { isApiError, isNetworkError, isUnauthenticated } from "@/api/client";
import { queryKeys } from "./keys";

const MAX_NETWORK_RETRIES = 2;

export function shouldRetry(failureCount: number, error: unknown): boolean {
  if (isApiError(error)) {
    return false;
  }
  return isNetworkError(error) && failureCount < MAX_NETWORK_RETRIES;
}

export function createQueryClient(): QueryClient {
  const client: QueryClient = new QueryClient({
    queryCache: new QueryCache({
      onError: (error, query) => {
        if (isUnauthenticated(error) && query.queryKey[0] !== queryKeys.me[0]) {
          client.setQueryData(queryKeys.me, null);
        }
      },
    }),
    mutationCache: new MutationCache({
      onError: (error) => {
        if (isUnauthenticated(error)) {
          client.setQueryData(queryKeys.me, null);
        }
      },
    }),
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: shouldRetry,
        refetchOnWindowFocus: true,
      },
    },
  });
  return client;
}
