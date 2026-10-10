import type { JSX } from "react";
import { Outlet, useParams } from "react-router";
import { workspacePath } from "@/api/paths";
import { describeError } from "@/api/errors";
import type { Workspace } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { TabNav, type TabItem } from "@/components/ui/tab-nav";
import { NotFoundPage } from "@/layout/not-found-page";
import { roleLabel } from "@/lib/roles";
import { usePageTitle } from "@/lib/use-page-title";
import { canInvite, canManageMembers, isWorkspaceAdmin } from "@/lib/workspace-access";
import { useInstance } from "@/queries/instance";
import { useMe } from "@/queries/session";
import { useWorkspaces } from "@/queries/workspaces";
import type { WorkspaceContext } from "./workspace-context";

function describe(workspace: Workspace): string {
  if (workspace.scopedOnly) {
    return "You can reach only some folders and sites in this workspace.";
  }
  return workspace.kind === "personal" ? "Your personal workspace." : `Shared workspace. Your role: ${roleLabel(workspace.role).toLowerCase()}.`;
}

function tabsFor(workspace: Workspace, isRoot: boolean, whoCanInvite: "root" | "workspaceAdmins" | undefined): TabItem[] {
  const base = workspacePath(workspace.slug);
  const tabs: TabItem[] = [{ to: base, label: "Sites", end: true }];
  if (canManageMembers(workspace)) {
    tabs.push({ to: `${base}/members`, label: "Members" });
  }
  if (canInvite(workspace, isRoot, whoCanInvite)) {
    tabs.push({ to: `${base}/invites`, label: "Invites" });
  }
  tabs.push({ to: `${base}/analytics`, label: "Analytics" });
  if (isWorkspaceAdmin(workspace)) {
    tabs.push({ to: `${base}/settings`, label: "Settings" });
  }
  return tabs;
}

export function WorkspaceLayout(): JSX.Element {
  const { workspace: slug = "" } = useParams();
  const workspaces = useWorkspaces();
  const me = useMe();
  const instance = useInstance();
  const workspace = workspaces.data?.find((item) => item.slug === slug || item.id === slug);
  usePageTitle(workspace?.name ?? "Workspace");

  if (workspaces.isError) {
    return <Alert tone="error">{describeError(workspaces.error).message}</Alert>;
  }
  if (workspaces.isPending) {
    return (
      <div role="status" aria-label="Loading workspace" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-64" />
        <Skeleton className="h-5 w-96 max-w-full" />
      </div>
    );
  }
  if (!workspace) {
    return <NotFoundPage title="Workspace not found" message="This workspace does not exist, or you do not have access to it." />;
  }

  const isRoot = me.data?.isRoot ?? false;
  const context: WorkspaceContext = { workspace, workspaces: workspaces.data, isRoot, userEmail: me.data?.email ?? "" };

  return (
    <>
      <PageHeader title={workspace.name} description={describe(workspace)} eyebrow="Workspace" />
      <TabNav label="Workspace sections" items={tabsFor(workspace, isRoot, instance.data?.whoCanInvite)} />
      <Outlet context={context} />
    </>
  );
}
