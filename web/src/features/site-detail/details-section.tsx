import { useState, type FormEvent, type JSX } from "react";
import type { Site } from "@/api/types";
import { Section } from "@/components/ui/section";
import { SaveBar } from "@/components/ui/save-bar";
import { TextAreaField, TextField } from "@/components/ui/field";
import { useToast } from "@/components/ui/toast";
import { FormError } from "@/features/auth/form-error";
import { changedFields } from "@/lib/diff";
import { canWrite } from "@/lib/roles";
import { useUpdateSite } from "@/queries/site-detail";

interface DraftDetails {
  displayName: string;
  description: string;
}

function DetailsForm({ site }: { site: Site }): JSX.Element {
  const update = useUpdateSite(site.slug);
  const toast = useToast();
  const original: DraftDetails = { displayName: site.displayName, description: site.description };
  const [draft, setDraft] = useState<DraftDetails>(original);
  const patch = changedFields(original, { displayName: draft.displayName.trim(), description: draft.description });
  const dirty = Object.keys(patch).length > 0;

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    update.mutate(patch, { onSuccess: () => toast.success("Site details saved.") });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <TextField
        label="Display name"
        value={draft.displayName}
        onChange={(event) => setDraft({ ...draft, displayName: event.target.value })}
        required
        maxLength={100}
        autoComplete="off"
        hint="Shown in the dashboard. The address does not change."
      />
      <TextAreaField
        label="Description"
        value={draft.description}
        onChange={(event) => setDraft({ ...draft, description: event.target.value })}
        maxLength={1000}
        hint="Optional. Only people with access to this site in the dashboard see it."
      />
      <FormError error={update.error} />
      <SaveBar dirty={dirty} pending={update.isPending} onDiscard={() => setDraft(original)} />
    </form>
  );
}

export function DetailsSection({ site }: { site: Site }): JSX.Element {
  const editable = canWrite(site.role);
  return (
    <Section title="Details" description="The name and description people see in the dashboard.">
      {editable ? (
        <DetailsForm site={site} />
      ) : (
        <dl className="grid gap-3 text-sm">
          <div>
            <dt className="text-xs font-medium uppercase tracking-wider text-muted">Display name</dt>
            <dd>{site.displayName}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wider text-muted">Description</dt>
            <dd>{site.description === "" ? <span className="text-muted">No description.</span> : site.description}</dd>
          </div>
          <p className="text-xs text-muted">You have read-only access to this site, so you cannot change these fields.</p>
        </dl>
      )}
    </Section>
  );
}
