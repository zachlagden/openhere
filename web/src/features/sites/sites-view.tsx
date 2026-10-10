import { PanelLeft } from "lucide-react";
import { useState, type JSX } from "react";
import type { Workspace } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { FolderNav } from "./folder-nav";
import { SearchInput } from "./search-input";
import { SiteList } from "./site-list";
import { useSitesParams } from "./use-sites-params";
import { WorkspaceNav } from "./workspace-nav";

function SidebarSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-2 px-2.5">
      <Skeleton className="h-5 w-3/4" />
      <Skeleton className="h-5 w-1/2" />
      <Skeleton className="h-5 w-2/3" />
    </div>
  );
}

interface SitesViewProps {
  workspace: Workspace | undefined;
  workspaces: Workspace[];
  loadingWorkspaces: boolean;
  isRoot: boolean;
}

export function SitesView({ workspace, workspaces, loadingWorkspaces, isRoot }: SitesViewProps): JSX.Element {
  const { params, update } = useSitesParams();
  const [navOpen, setNavOpen] = useState(false);
  const effective = { q: params.q, folder: workspace ? params.folder : "" };

  return (
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
          {loadingWorkspaces ? <SidebarSkeleton /> : workspace ? <FolderNav workspace={workspace} activeFolder={effective.folder} /> : <WorkspaceNav workspaces={workspaces} />}
        </div>
      </aside>
      <section aria-label="Sites" className="flex min-w-0 flex-col gap-4">
        <SearchInput value={effective.q} onSearch={(q) => update({ q })} />
        <SiteList
          workspace={workspace?.slug ?? ""}
          params={effective}
          workspaces={workspaces}
          isRoot={isRoot}
          onClearFilters={() => update({ q: "", folder: "" })}
        />
      </section>
    </div>
  );
}
