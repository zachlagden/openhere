import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import * as workspaceApi from "@/api/workspaces";
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

export function useRenameWorkspace(slug: string): UseMutationResult<Workspace, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => workspaceApi.renameWorkspace(slug, name),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.workspaces, exact: true }),
  });
}

export function useDeleteWorkspace(slug: string): UseMutationResult<void, Error, void> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: () => workspaceApi.deleteWorkspace(slug),
    onSuccess: async () => {
      await Promise.all([client.invalidateQueries({ queryKey: queryKeys.workspaces, exact: true }), client.invalidateQueries({ queryKey: queryKeys.sites })]);
    },
  });
}
