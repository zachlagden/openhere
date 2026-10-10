import { Folder as FolderIcon, FolderOpen, FolderPlus, Inbox, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { useState, type JSX } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import type { Folder, Workspace } from "@/api/types";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { describeError } from "@/api/errors";
import { cn } from "@/lib/cn";
import { UNFILED_FOLDER, writeSitesParams } from "@/lib/query-state";
import { canDeleteFolders, canRenameFolders, canWriteWholeWorkspace } from "@/lib/roles";
import { useFolders } from "@/queries/folders";
import { CreateFolderDialog, DeleteFolderDialog, RenameFolderDialog } from "./folder-dialogs";

type FolderDialog = { kind: "create" } | { kind: "rename"; folder: Folder } | { kind: "delete"; folder: Folder } | null;

interface NavLinkProps {
  active: boolean;
  to: string;
  icon: JSX.Element;
  children: string;
}

function NavItem({ active, to, icon, children }: NavLinkProps): JSX.Element {
  return (
    <Link
      to={to}
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex min-w-0 flex-1 items-center gap-2 rounded-md px-2.5 py-2 text-sm hover:bg-sunken [&_svg]:size-4 [&_svg]:shrink-0",
        active ? "bg-accent-soft font-medium text-foreground" : "text-muted hover:text-foreground",
      )}
    >
      {icon}
      <span className="truncate">{children}</span>
    </Link>
  );
}

export function FolderNav({ workspace, activeFolder }: { workspace: Workspace; activeFolder: string }): JSX.Element {
  const folders = useFolders(workspace.slug);
  const [search] = useSearchParams();
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<FolderDialog>(null);
  const base = `/workspaces/${encodeURIComponent(workspace.slug)}`;

  function target(folder: string): string {
    const query = writeSitesParams(search, { folder }).toString();
    return query === "" ? base : `${base}?${query}`;
  }

  function closeDialog(open: boolean): void {
    if (!open) {
      setDialog(null);
    }
  }

  return (
    <nav aria-label="Folders" className="flex flex-col gap-0.5">
      <div className="flex">
        <NavItem active={activeFolder === ""} to={target("")} icon={<FolderOpen aria-hidden="true" />}>
          All sites
        </NavItem>
      </div>
      <div className="flex">
        <NavItem active={activeFolder === UNFILED_FOLDER} to={target(UNFILED_FOLDER)} icon={<Inbox aria-hidden="true" />}>
          Unfiled
        </NavItem>
      </div>
      {folders.isPending ? (
        <div className="flex flex-col gap-2 px-2.5 py-2">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </div>
      ) : null}
      {folders.isError ? <p className="px-2.5 py-2 text-xs text-danger">{describeError(folders.error).message}</p> : null}
      {(folders.data ?? []).map((folder) => (
        <div key={folder.id} className="group flex items-center">
          <NavItem active={activeFolder === folder.id} to={target(folder.id)} icon={<FolderIcon aria-hidden="true" />}>
            {folder.name}
          </NavItem>
          {canRenameFolders(workspace) || canDeleteFolders(workspace) ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="size-8 shrink-0" aria-label={`Options for folder ${folder.name}`}>
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {canRenameFolders(workspace) ? (
                  <DropdownMenuItem onSelect={() => setDialog({ kind: "rename", folder })}>
                    <Pencil /> Rename
                  </DropdownMenuItem>
                ) : null}
                {canDeleteFolders(workspace) ? (
                  <DropdownMenuItem onSelect={() => setDialog({ kind: "delete", folder })}>
                    <Trash2 /> Delete
                  </DropdownMenuItem>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
        </div>
      ))}
      {canWriteWholeWorkspace(workspace) ? (
        <Button variant="ghost" size="sm" className="mt-1 justify-start text-muted" onClick={() => setDialog({ kind: "create" })}>
          <FolderPlus /> New folder
        </Button>
      ) : null}
      {dialog?.kind === "create" ? (
        <CreateFolderDialog workspace={workspace.slug} open onOpenChange={closeDialog} />
      ) : null}
      {dialog?.kind === "rename" ? <RenameFolderDialog workspace={workspace.slug} folder={dialog.folder} open onOpenChange={closeDialog} /> : null}
      {dialog?.kind === "delete" ? (
        <DeleteFolderDialog
          workspace={workspace.slug}
          folder={dialog.folder}
          open
          onOpenChange={closeDialog}
          onDeleted={() => {
            if (activeFolder === dialog.folder.id) {
              void navigate(target(""), { replace: true });
            }
          }}
        />
      ) : null}
    </nav>
  );
}
