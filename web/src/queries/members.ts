import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/workspaces";
import type { AddMemberRequest, Member, Role } from "@/api/types";
import { queryKeys } from "./keys";

export function useMembers(workspace: string, enabled = true): UseQueryResult<Member[]> {
  return useQuery({
    queryKey: queryKeys.members(workspace),
    enabled,
    queryFn: ({ signal }) => api.listMembers(workspace, { signal }),
  });
}

function useRefreshMembers(workspace: string): () => Promise<void> {
  const client = useQueryClient();
  return async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: queryKeys.members(workspace) }),
      client.invalidateQueries({ queryKey: queryKeys.workspaces, exact: true }),
    ]);
  };
}

export function useAddMember(workspace: string): UseMutationResult<Member, Error, AddMemberRequest> {
  const refresh = useRefreshMembers(workspace);
  return useMutation({ mutationFn: (body: AddMemberRequest) => api.addMember(workspace, body), onSuccess: refresh });
}

export function useUpdateMember(workspace: string): UseMutationResult<Member, Error, { id: string; role: Role }> {
  const refresh = useRefreshMembers(workspace);
  return useMutation({ mutationFn: ({ id, role }) => api.updateMember(workspace, id, role), onSuccess: refresh });
}

export function useRemoveMember(workspace: string): UseMutationResult<void, Error, string> {
  const refresh = useRefreshMembers(workspace);
  return useMutation({ mutationFn: (id: string) => api.removeMember(workspace, id), onSuccess: refresh });
}
