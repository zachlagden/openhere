import type { JSX } from "react";
import { SitesView } from "@/features/sites/sites-view";
import { useWorkspaceContext } from "./workspace-context";

export function WorkspaceSitesTab(): JSX.Element {
  const { workspace, workspaces, isRoot } = useWorkspaceContext();
  return <SitesView workspace={workspace} workspaces={workspaces} loadingWorkspaces={false} isRoot={isRoot} />;
}
