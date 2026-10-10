import { useOutletContext } from "react-router";
import type { Workspace } from "@/api/types";

export interface WorkspaceContext {
  workspace: Workspace;
  workspaces: Workspace[];
  isRoot: boolean;
  userEmail: string;
}

export function useWorkspaceContext(): WorkspaceContext {
  return useOutletContext<WorkspaceContext>();
}
