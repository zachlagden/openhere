import { useState, type FormEvent, type JSX } from "react";
import type { Site } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Dialog, DialogFooter } from "@/components/ui/dialog";
import { SelectField } from "@/components/ui/field";
import { useToast } from "@/components/ui/toast";
import { FormError } from "@/features/auth/form-error";
import { useFolders } from "@/queries/folders";
import { useMoveSite } from "@/queries/sites";

interface MoveSiteDialogProps {
  site: Site;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const UNFILED = "";

export function MoveSiteDialog({ site, open, onOpenChange }: MoveSiteDialogProps): JSX.Element {
  const folders = useFolders(site.workspaceSlug);
  const move = useMoveSite();
  const toast = useToast();
  const [target, setTarget] = useState(site.folderId ?? UNFILED);

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    move.mutate(
      { slug: site.slug, folderId: target === UNFILED ? null : target },
      {
        onSuccess: () => {
          onOpenChange(false);
          toast.success(`Moved ${site.displayName}.`);
        },
      },
    );
  }

  const unchanged = target === (site.folderId ?? UNFILED);

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Move to folder" description={`Choose where ${site.displayName} lives in its workspace.`}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        <SelectField label="Folder" value={target} onChange={(event) => setTarget(event.target.value)} disabled={folders.isPending}>
          <option value={UNFILED}>No folder (unfiled)</option>
          {(folders.data ?? []).map((folder) => (
            <option key={folder.id} value={folder.id}>
              {folder.name}
            </option>
          ))}
        </SelectField>
        {folders.isError ? <FormError error={folders.error} /> : null}
        <FormError error={move.error} />
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={move.isPending || unchanged}>
            {move.isPending ? "Moving…" : "Move site"}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}
