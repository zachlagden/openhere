import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/workspaces";
import type { CreatedInvite, CreateInviteRequest, Invite } from "@/api/types";
import { queryKeys } from "./keys";

export function useInvites(enabled = true): UseQueryResult<Invite[]> {
  return useQuery({
    queryKey: queryKeys.invites,
    enabled,
    queryFn: ({ signal }) => api.listInvites({ signal }),
  });
}

export function useCreateInvite(): UseMutationResult<CreatedInvite, Error, CreateInviteRequest> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.createInvite,
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.invites }),
  });
}

export function useRevokeInvite(): UseMutationResult<void, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.revokeInvite,
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.invites }),
  });
}

export function useResendInvite(): UseMutationResult<CreatedInvite, Error, { id: string; sendEmail: boolean }> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ id, sendEmail }) => api.resendInvite(id, sendEmail),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.invites }),
  });
}
