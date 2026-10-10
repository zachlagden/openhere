import { PanelLeft } from "lucide-react";
import { useState, type JSX } from "react";
import { useParams } from "react-router";
import { describeError } from "@/api/errors";
import type { Workspace } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { NotFoundPage } from "@/layout/not-found-page";
import { roleLabel } from "@/lib/roles";
import { useMe } from "@/queries/session";
import { useWorkspaces } from "@/queries/workspaces";
import { FolderNav } from "./folder-nav";
import { SearchInput } from "./search-input";
import { SiteList } from "./site-list";
import { useSitesParams } from "./use-sites-params";
import { WorkspaceNav } from "./workspace-nav";

function workspaceDescription(workspace: Workspace): string {
  if (workspace.scopedOnly) {
    return "You can reach only some folders and sites in this workspace.";
  }
  return workspace.kind === "personal" ? "Your personal workspace." : `Shared workspace. Your role: ${roleLabel(workspace.role).toLowerCase()}.`;
}

function SidebarSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-2 px-2.5">
      <Skeleton className="h-5 w-3/4" />
      <Skeleton className="h-5 w-1/2" />
      <Skeleton className="h-5 w-2/3" />
    </div>
  );
}

export function SitesPage(): JSX.Element {
  const { workspace: slug } = useParams();
  const { params, update } = useSitesParams();
  const workspaces = useWorkspaces();
  const me = useMe();
  const [navOpen, setNavOpen] = useState(false);

  if (workspaces.isError) {
    return <Alert tone="error">{describeError(workspaces.error).message}</Alert>;
  }

  const list = workspaces.data ?? [];
  const workspace = slug === undefined ? undefined : list.find((item) => item.slug === slug || item.id === slug);
  if (slug !== undefined && workspaces.data && !workspace) {
    return <NotFoundPage title="Workspace not found" message="This workspace does not exist, or you do not have access to it." />;
  }

  const effective = { q: params.q, folder: workspace ? params.folder : "" };

  return (
    <>
      <PageHeader
        title={workspace ? workspace.name : "Sites"}
        description={workspace ? workspaceDescription(workspace) : "Every site you can reach, across all your workspaces."}
        eyebrow={workspace ? "Workspace" : undefined}
      />
      <div className="grid gap-x-6 gap-y-3 lg:grid-cols-[15rem_minmax(0,1fr)]">
        <aside className="min-w-0">
          <Button
            variant="secondary"
            size="sm"
            className="mb-3 w-full justify-start lg:hidden"
            aria-expanded={navOpen}
            aria-controls="sites-nav"
            onClick={() => setNavOpen((open) => !open)}
          >
            <PanelLeft /> {workspace ? "Folders" : "Workspaces"}
          </Button>
          <div id="sites-nav" className={navOpen ? "block" : "hidden lg:block"}>
            {workspaces.isPending ? <SidebarSkeleton /> : workspace ? <FolderNav workspace={workspace} activeFolder={effective.folder} /> : <WorkspaceNav workspaces={list} />}
          </div>
        </aside>
        <section aria-label="Sites" className="flex min-w-0 flex-col gap-4">
          <SearchInput value={effective.q} onSearch={(q) => update({ q })} />
          <SiteList
            workspace={workspace?.slug ?? ""}
            params={effective}
            workspaces={list}
            isRoot={me.data?.isRoot ?? false}
            onClearFilters={() => update({ q: "", folder: "" })}
          />
        </section>
      </div>
    </>
  );
}
