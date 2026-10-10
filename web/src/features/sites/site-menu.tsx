import { FolderInput, MoreHorizontal } from "lucide-react";
import { useState, type JSX } from "react";
import type { Site } from "@/api/types";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { canWrite } from "@/lib/roles";
import { MoveSiteDialog } from "./move-site-dialog";

export function SiteMenu({ site }: { site: Site }): JSX.Element | null {
  const [moving, setMoving] = useState(false);
  if (!canWrite(site.role)) {
    return null;
  }
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="size-8" aria-label={`Actions for ${site.displayName}`}>
            <MoreHorizontal />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={() => setMoving(true)}>
            <FolderInput /> Move to folder
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {moving ? <MoveSiteDialog site={site} open={moving} onOpenChange={setMoving} /> : null}
    </>
  );
}
