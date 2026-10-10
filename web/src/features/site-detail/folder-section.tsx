import { useState, type FormEvent, type JSX } from "react";
import type { Site } from "@/api/types";
import { Button } from "@/components/ui/button";
import { SelectField } from "@/components/ui/field";
import { Section } from "@/components/ui/section";
import { useToast } from "@/components/ui/toast";
import { FormError } from "@/features/auth/form-error";
import { canWrite } from "@/lib/roles";
import { useFolders } from "@/queries/folders";
import { useUpdateSite } from "@/queries/site-detail";

const UNFILED = "";

function FolderForm({ site }: { site: Site }): JSX.Element {
  const folders = useFolders(site.workspaceSlug);
  const update = useUpdateSite(site.slug);
  const toast = useToast();
  const current = site.folderId ?? UNFILED;
  const [target, setTarget] = useState(current);

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    update.mutate({ folderId: target === UNFILED ? null : target }, { onSuccess: () => toast.success("Site moved.") });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3 sm:flex-row sm:items-end">
      <div className="sm:w-72">
        <SelectField label="Folder" value={target} onChange={(event) => setTarget(event.target.value)} disabled={folders.isPending}>
          <option value={UNFILED}>No folder (unfiled)</option>
          {(folders.data ?? []).map((folder) => (
            <option key={folder.id} value={folder.id}>
              {folder.name}
            </option>
          ))}
        </SelectField>
      </div>
      <Button type="submit" variant="primary" disabled={update.isPending || target === current}>
        {update.isPending ? "Moving…" : "Move site"}
      </Button>
      <div className="sm:basis-full">
        <FormError error={update.error ?? folders.error} />
      </div>
    </form>
  );
}

export function FolderSection({ site }: { site: Site }): JSX.Element {
  return (
    <Section title="Folder" description="Folders group sites inside one workspace. A site cannot move to another workspace.">
      {canWrite(site.role) ? <FolderForm site={site} /> : <p className="text-sm">Folders are changed by people with write access.</p>}
    </Section>
  );
}
