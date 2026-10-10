import { useMutation, useQuery, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import type { AcceptInviteRequest, InvitePreview, PasswordResetConfirm, User } from "@/api/types";
import { useSignedIn } from "./session";

export function useInvite(token: string): UseQueryResult<InvitePreview> {
  return useQuery({
    queryKey: ["invite", token],
    staleTime: 0,
    queryFn: ({ signal }) => api.getInvite(token, { signal }),
  });
}

export function useAcceptInvite(token: string): UseMutationResult<User, Error, AcceptInviteRequest> {
  const signedIn = useSignedIn();
  return useMutation({
    mutationFn: (body: AcceptInviteRequest) => api.acceptInvite(token, body),
    onSuccess: signedIn,
  });
}

export function useRequestPasswordReset(): UseMutationResult<void, Error, string> {
  return useMutation({ mutationFn: api.requestPasswordReset });
}

export function useConfirmPasswordReset(): UseMutationResult<void, Error, PasswordResetConfirm> {
  return useMutation({ mutationFn: api.confirmPasswordReset });
}
