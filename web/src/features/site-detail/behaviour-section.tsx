import type { JSX } from "react";
import type { Site, UpdateSiteRequest } from "@/api/types";
import { describeError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { Section } from "@/components/ui/section";
import { SwitchField } from "@/components/ui/switch";
import { useToast } from "@/components/ui/toast";
import { canWrite } from "@/lib/roles";
import { useUpdateSite } from "@/queries/site-detail";

export function BehaviourSection({ site }: { site: Site }): JSX.Element {
  const update = useUpdateSite(site.slug);
  const toast = useToast();
  const editable = canWrite(site.role);

  function change(patch: UpdateSiteRequest, message: string): void {
    update.mutate(patch, { onSuccess: () => toast.success(message) });
  }

  return (
    <Section title="Behaviour" description="How the server serves this site and what it counts.">
      <SwitchField
        label="Single-page app mode"
        description="Serve index.html with status 200 for unknown paths that have no file extension. Turn it on for sites that route in the browser."
        checked={site.spaMode}
        disabled={!editable || update.isPending}
        onChange={(event) => change({ spaMode: event.target.checked }, event.target.checked ? "Single-page app mode is on." : "Single-page app mode is off.")}
      />
      <SwitchField
        label="Count page views"
        description="Records anonymous page views for the Analytics tab. Turning it off stops counting and keeps what was recorded."
        checked={site.analyticsEnabled}
        disabled={!editable || update.isPending}
        onChange={(event) => change({ analyticsEnabled: event.target.checked }, event.target.checked ? "Page views are counted." : "Page views are no longer counted.")}
      />
      {update.isError ? <Alert tone="error">{describeError(update.error).message}</Alert> : null}
      {editable ? null : <p className="text-xs text-muted">You have read-only access to this site, so you cannot change these options.</p>}
    </Section>
  );
}
