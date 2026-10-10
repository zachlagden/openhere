import { History } from "lucide-react";
import { useState, type JSX } from "react";
import { describeError } from "@/api/errors";
import type { Version } from "@/api/types";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import { useToast } from "@/components/ui/toast";
import { canWrite } from "@/lib/roles";
import { versionLabel } from "@/lib/versions";
import { useMe } from "@/queries/session";
import { useDeleteVersion, useRestoreVersion, useVersions } from "@/queries/site-detail";
import { LiveFiles } from "./live-files";
import { useSiteContext } from "./site-context";
import { VersionsTable } from "./versions-table";

type Pending = { kind: "restore"; version: Version } | { kind: "delete"; version: Version } | null;

export function VersionsTab(): JSX.Element {
  const { site } = useSiteContext();
  const versions = useVersions(site.slug);
  const me = useMe();
  const restore = useRestoreVersion(site.slug);
  const remove = useDeleteVersion(site.slug);
  const toast = useToast();
  const [pending, setPending] = useState<Pending>(null);
  const editable = canWrite(site.role);

  function close(): void {
    setPending(null);
    restore.reset();
    remove.reset();
  }

  function confirm(): void {
    if (pending === null) {
      return;
    }
    const label = versionLabel(pending.version);
    if (pending.kind === "restore") {
      restore.mutate(pending.version.id, {
        onSuccess: () => {
          close();
          toast.success(`Version ${label} is live again.`);
        },
      });
    } else {
      remove.mutate(pending.version.id, {
        onSuccess: () => {
          close();
          toast.success(`Deleted version ${label}.`);
        },
      });
    }
  }

  const failure = pending?.kind === "restore" ? restore.error : remove.error;

  return (
    <div className="flex flex-col gap-6">
      <Section
        title="Version history"
        description="Every publish creates a version. Restoring one makes it live at once and uploads nothing. The server keeps a limited number of versions per site."
      >
        {versions.isPending ? <BlockSkeleton label="Loading versions" rows={3} /> : null}
        {versions.isError ? <LoadError error={versions.error} onRetry={() => void versions.refetch()} /> : null}
        {versions.data && versions.data.length === 0 ? (
          <EmptyState icon={<History />} title="No versions yet">
            <p>Nothing has been published to this site. Your agent publishes through the API, and each publish appears here.</p>
          </EmptyState>
        ) : null}
        {versions.data && versions.data.length > 0 ? (
          <VersionsTable
            versions={versions.data}
            currentUserId={me.data?.id}
            editable={editable}
            onRestore={(version) => setPending({ kind: "restore", version })}
            onDelete={(version) => setPending({ kind: "delete", version })}
          />
        ) : null}
        {editable ? null : <p className="text-sm text-muted">You have read-only access to this site, so you cannot restore or delete versions.</p>}
      </Section>
      <LiveFiles slug={site.slug} />
      <ConfirmDialog
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open) {
            close();
          }
        }}
        title={pending?.kind === "restore" ? `Restore version ${versionLabel(pending.version)}?` : `Delete version ${pending ? versionLabel(pending.version) : ""}?`}
        confirmLabel={pending?.kind === "restore" ? "Restore version" : "Delete version"}
        tone={pending?.kind === "restore" ? "primary" : "danger"}
        pending={restore.isPending || remove.isPending}
        error={failure ? describeError(failure).message : null}
        onConfirm={confirm}
      >
        {pending?.kind === "restore" ? (
          <p>This version becomes the one visitors see right away. The current version stays in the history, so you can restore it again.</p>
        ) : (
          <p>The version and its files are deleted for good. The live version is not affected. This cannot be undone.</p>
        )}
      </ConfirmDialog>
    </div>
  );
}
