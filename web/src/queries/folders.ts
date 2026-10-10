import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import type { Folder } from "@/api/types";
import { queryKeys } from "./keys";

export function useFolders(workspace: string | undefined): UseQueryResult<Folder[]> {
  return useQuery({
    queryKey: queryKeys.folders(workspace ?? ""),
    enabled: workspace !== undefined && workspace !== "",
    queryFn: ({ signal }) => api.listFolders(workspace ?? "", { signal }),
  });
}

export function useCreateFolder(workspace: string): UseMutationResult<Folder, Error, string> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => api.createFolder(workspace, name),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.folders(workspace) }),
  });
}

export function useRenameFolder(workspace: string): UseMutationResult<Folder, Error, { id: string; name: string }> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ id, name }) => api.renameFolder(workspace, id, name),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.folders(workspace) }),
  });
}

export function useDeleteFolder(workspace: string): UseMutationResult<void, Error, { id: string; force: boolean }> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ id, force }) => api.deleteFolder(workspace, id, force),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: queryKeys.folders(workspace) }),
        client.invalidateQueries({ queryKey: queryKeys.sites }),
      ]);
    },
  });
}
