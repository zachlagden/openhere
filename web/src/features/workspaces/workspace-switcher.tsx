import { Check, ChevronsUpDown, Layers, Plus, User } from "lucide-react";
import { useState, type JSX } from "react";
import { Link, useMatch } from "react-router";
import type { Workspace } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { roleLabel } from "@/lib/roles";
import { useInstance } from "@/queries/instance";
import { useWorkspaces } from "@/queries/workspaces";
import { CreateWorkspaceDialog } from "./create-workspace-dialog";

function WorkspaceMark({ workspace }: { workspace: Workspace }): JSX.Element {
  return workspace.kind === "personal" ? <User aria-hidden="true" /> : <Layers aria-hidden="true" />;
}

export function WorkspaceSwitcher(): JSX.Element {
  const match = useMatch("/workspaces/:workspace/*");
  const currentSlug = match?.params.workspace;
  const workspaces = useWorkspaces();
  const instance = useInstance();
  const [creating, setCreating] = useState(false);

  const list = workspaces.data ?? [];
  const current = list.find((workspace) => workspace.slug === currentSlug);
  const mayCreate = instance.data?.canCreateWorkspaces ?? true;

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" className="min-w-0 max-w-[12rem] justify-between gap-1.5 px-2.5 sm:max-w-[16rem]" aria-label="Switch workspace">
            {current ? <WorkspaceMark workspace={current} /> : <Layers aria-hidden="true" />}
            <span className="truncate">{current ? current.name : currentSlug ? currentSlug : "All workspaces"}</span>
            <ChevronsUpDown aria-hidden="true" className="text-muted" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-72">
          <DropdownMenuLabel>Workspaces</DropdownMenuLabel>
          <DropdownMenuItem asChild>
            <Link to="/">
              <Layers aria-hidden="true" />
              <span className="flex-1">All workspaces</span>
              {currentSlug === undefined ? <Check aria-hidden="true" /> : null}
            </Link>
          </DropdownMenuItem>
          {workspaces.isPending ? <p className="px-2.5 py-2 text-xs text-muted">Loading workspaces…</p> : null}
          {workspaces.isError ? <p className="px-2.5 py-2 text-xs text-danger">Could not load workspaces.</p> : null}
          {list.map((workspace) => (
            <DropdownMenuItem asChild key={workspace.id}>
              <Link to={`/workspaces/${encodeURIComponent(workspace.slug)}`}>
                <WorkspaceMark workspace={workspace} />
                <span className="min-w-0 flex-1 truncate">{workspace.name}</span>
                {workspace.scopedOnly ? <Badge title="You can reach only some folders or sites here">Limited</Badge> : <Badge>{roleLabel(workspace.role)}</Badge>}
                {workspace.slug === currentSlug ? <Check aria-hidden="true" /> : null}
              </Link>
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuItem disabled={!mayCreate} onSelect={() => setCreating(true)}>
            <Plus aria-hidden="true" /> Create shared workspace
          </DropdownMenuItem>
          {mayCreate ? null : (
            <p className="px-2.5 pb-2 text-xs text-muted">Only the root user can create shared workspaces on this server.</p>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <CreateWorkspaceDialog open={creating} onOpenChange={setCreating} />
    </>
  );
}
