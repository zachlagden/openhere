import { Layers, User } from "lucide-react";
import type { JSX } from "react";
import { Link } from "react-router";
import type { Workspace } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { roleLabel } from "@/lib/roles";

export function WorkspaceNav({ workspaces }: { workspaces: Workspace[] }): JSX.Element {
  return (
    <nav aria-label="Workspaces" className="flex flex-col gap-0.5">
      {workspaces.map((workspace) => (
        <Link
          key={workspace.id}
          to={`/workspaces/${encodeURIComponent(workspace.slug)}`}
          className="flex min-w-0 items-center gap-2 rounded-md px-2.5 py-2 text-sm text-muted hover:bg-sunken hover:text-foreground [&_svg]:size-4 [&_svg]:shrink-0"
        >
          {workspace.kind === "personal" ? <User aria-hidden="true" /> : <Layers aria-hidden="true" />}
          <span className="min-w-0 flex-1 truncate">{workspace.name}</span>
          <Badge>{workspace.scopedOnly ? "Limited" : roleLabel(workspace.role)}</Badge>
        </Link>
      ))}
    </nav>
  );
}
