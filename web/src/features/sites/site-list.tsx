import { SearchX, Sprout } from "lucide-react";
import { useMemo, type JSX } from "react";
import { describeError } from "@/api/errors";
import type { Workspace } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { pluralise } from "@/lib/format";
import { useNow } from "@/lib/use-now";
import { hasActiveFilters, type SitesParams } from "@/lib/query-state";
import { useCopySetupPrompt } from "@/layout/use-copy-setup-prompt";
import { useSites } from "@/queries/sites";
import { SiteRow } from "./site-row";
import { useFolderNames } from "./use-folder-names";

interface SiteListProps {
  workspace: string;
  params: SitesParams;
  workspaces: Workspace[];
  isRoot: boolean;
  onClearFilters: () => void;
}

function SiteListSkeleton(): JSX.Element {
  return (
    <ul aria-label="Loading sites" className="divide-y divide-line rounded-lg border border-line bg-surface">
      {[0, 1, 2, 3].map((index) => (
        <li key={index} className="flex flex-col gap-3 px-5 py-4">
          <Skeleton className="h-5 w-48" />
          <Skeleton className="h-4 w-72 max-w-full" />
        </li>
      ))}
    </ul>
  );
}

export function SiteList({ workspace, params, workspaces, isRoot, onClearFilters }: SiteListProps): JSX.Element {
  const sites = useSites({ workspace, folder: params.folder, q: params.q });
  const folderNames = useFolderNames(workspaces);
  const copyPrompt = useCopySetupPrompt();
  const workspaceNames = useMemo(() => new Map(workspaces.map((item) => [item.id, item.name])), [workspaces]);
  const now = useNow();

  if (sites.isPending) {
    return <SiteListSkeleton />;
  }
  if (sites.isError && !sites.data) {
    return (
      <Alert tone="error">
        <p>{describeError(sites.error).message}</p>
        <Button variant="secondary" size="sm" className="mt-2" onClick={() => void sites.refetch()}>
          Try again
        </Button>
      </Alert>
    );
  }

  const rows = sites.data.pages.flatMap((page) => page.sites);
  if (rows.length === 0) {
    if (hasActiveFilters(params)) {
      return (
        <EmptyState
          icon={<SearchX />}
          title="No sites match"
          actions={
            <Button variant="secondary" onClick={onClearFilters}>
              Clear search and filters
            </Button>
          }
        >
          <p>Nothing in this view matches your search or folder. Search covers site names and addresses.</p>
        </EmptyState>
      );
    }
    return (
      <EmptyState
        icon={<Sprout />}
        title="No sites yet"
        actions={
          <Button variant="primary" onClick={() => void copyPrompt()}>
            Copy agent setup prompt
          </Button>
        }
      >
        <p>Your agent publishes sites through the API, and they appear here. Copy the setup prompt and paste it into your agent to begin.</p>
      </EmptyState>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <p aria-live="polite" className="text-sm text-muted">
        {sites.hasNextPage ? `Showing ${rows.length} sites` : pluralise(rows.length, "site")}
        {sites.isFetching && !sites.isFetchingNextPage ? " (updating)" : ""}
      </p>
      <ul className="divide-y divide-line overflow-hidden rounded-lg border border-line bg-surface">
        {rows.map((site) => (
          <SiteRow
            key={site.id}
            site={site}
            workspaceName={workspaceNames.get(site.workspaceId) ?? site.workspaceSlug}
            folderName={site.folderId ? (folderNames.get(site.folderId) ?? null) : null}
            isRoot={isRoot}
            now={now}
          />
        ))}
      </ul>
      {sites.isError ? <Alert tone="error">{describeError(sites.error).message}</Alert> : null}
      {sites.hasNextPage ? (
        <div className="flex justify-center">
          <Button variant="secondary" disabled={sites.isFetchingNextPage} onClick={() => void sites.fetchNextPage()}>
            {sites.isFetchingNextPage ? "Loading…" : "Load more sites"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
