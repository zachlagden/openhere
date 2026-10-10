import { useState, type JSX } from "react";
import { useNavigate } from "react-router";
import { workspacePath } from "@/api/paths";
import { describeError } from "@/api/errors";
import type { Site } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Section } from "@/components/ui/section";
import { useToast } from "@/components/ui/toast";
import { TypedConfirmDialog } from "@/components/ui/typed-confirm-dialog";
import { canWrite } from "@/lib/roles";
import { useDeleteSite } from "@/queries/site-detail";

export function DeleteSiteSection({ site }: { site: Site }): JSX.Element | null {
  const [open, setOpen] = useState(false);
  const remove = useDeleteSite(site.slug);
  const navigate = useNavigate();
  const toast = useToast();

  if (!canWrite(site.role)) {
    return null;
  }

  async function confirm(): Promise<void> {
    try {
      await remove.mutateAsync();
    } catch {
      return;
    }
    setOpen(false);
    toast.success(`Deleted ${site.displayName}.`);
    void navigate(workspacePath(site.workspaceSlug), { replace: true });
  }

  return (
    <Section title="Delete site" description="Removes the site, its versions and its custom domains. The address stops working at once.">
      <div>
        <Button variant="danger" onClick={() => setOpen(true)}>
          Delete this site
        </Button>
      </div>
      <TypedConfirmDialog
        open={open}
        onOpenChange={(next) => {
          if (!next) {
            remove.reset();
          }
          setOpen(next);
        }}
        title={`Delete ${site.displayName}?`}
        phrase={site.slug}
        confirmLabel="Delete site"
        pending={remove.isPending}
        error={remove.error ? describeError(remove.error).message : null}
        onConfirm={() => void confirm()}
      >
        <p>This permanently deletes the site and every version of it. Visitors get a not-found page. This cannot be undone.</p>
      </TypedConfirmDialog>
    </Section>
  );
}
