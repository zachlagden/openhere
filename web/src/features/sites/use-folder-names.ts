import { useQueries, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import type { Folder, Workspace } from "@/api/types";
import { queryKeys } from "@/queries/keys";

function combineFolderNames(results: UseQueryResult<Folder[]>[]): ReadonlyMap<string, string> {
  const names = new Map<string, string>();
  for (const result of results) {
    for (const folder of result.data ?? []) {
      names.set(folder.id, folder.name);
    }
  }
  return names;
}

export function useFolderNames(workspaces: Workspace[]): ReadonlyMap<string, string> {
  return useQueries({
    queries: workspaces.map((workspace) => ({
      queryKey: queryKeys.folders(workspace.slug),
      queryFn: ({ signal }: { signal: AbortSignal }) => api.listFolders(workspace.slug, { signal }),
    })),
    combine: combineFolderNames,
  });
}
