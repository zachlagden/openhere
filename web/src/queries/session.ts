import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import { isUnauthenticated } from "@/api/client";
import type { LoginRequest, SetupRequest, User } from "@/api/types";
import { queryKeys } from "./keys";

export function useMe(): UseQueryResult<User | null> {
  return useQuery({
    queryKey: queryKeys.me,
    staleTime: 5 * 60_000,
    queryFn: async ({ signal }) => {
      try {
        return await api.getMe({ signal });
      } catch (error) {
        if (isUnauthenticated(error)) {
          return null;
        }
        throw error;
      }
    },
  });
}

export function useSetupStatus(enabled = true): UseQueryResult<boolean> {
  return useQuery({
    queryKey: queryKeys.setupStatus,
    enabled,
    staleTime: 0,
    queryFn: ({ signal }) => api.getSetupStatus({ signal }),
  });
}

export function useSignedIn(): (user: User) => Promise<void> {
  const client = useQueryClient();
  return async (user) => {
    client.setQueryData(queryKeys.me, user);
    await client.invalidateQueries({ predicate: (query) => query.queryKey[0] !== queryKeys.me[0] });
  };
}

export function useLogin(): UseMutationResult<User, Error, LoginRequest> {
  const signedIn = useSignedIn();
  return useMutation({
    mutationFn: api.login,
    onSuccess: signedIn,
  });
}

export function useSetup(): UseMutationResult<User, Error, SetupRequest> {
  const client = useQueryClient();
  const signedIn = useSignedIn();
  return useMutation({
    mutationFn: api.setup,
    onSuccess: async (user) => {
      client.setQueryData(queryKeys.setupStatus, false);
      await signedIn(user);
    },
  });
}

export function useLogout(): UseMutationResult<void, Error, void> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      client.clear();
      client.setQueryData(queryKeys.me, null);
    },
  });
}
