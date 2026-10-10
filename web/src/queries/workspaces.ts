import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import type { Workspace } from "@/api/types";
import { queryKeys } from "./keys";

export function useWorkspaces(): UseQueryResult<Workspace[]> {
  return useQuery({
    queryKey: queryKeys.workspaces,
    queryFn: ({ signal }) => api.listWorkspaces({ signal }),
  });
}

export function useCreateWorkspace(): UseMutationResult<Workspace, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.createWorkspace,
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.workspaces, exact: true }),
  });
}
