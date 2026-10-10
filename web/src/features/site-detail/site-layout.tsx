import type { JSX } from "react";
import { Link, Outlet, useParams } from "react-router";
import { isApiError } from "@/api/client";
import { sitePath, workspacePath } from "@/api/paths";
import { PageHeader } from "@/components/ui/page-header";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { TabNav } from "@/components/ui/tab-nav";
import { HostingNotices } from "@/features/sites/hosting-notices";
import { SiteAddress } from "@/features/sites/site-address";
import { AccessBadge, AnalyticsBadge, PublishBadge } from "@/features/sites/site-badges";
import { NotFoundPage } from "@/layout/not-found-page";
import { usePageTitle } from "@/lib/use-page-title";
import { useFolders } from "@/queries/folders";
import { useMe } from "@/queries/session";
import { useSite } from "@/queries/site-detail";
import { useWorkspaces } from "@/queries/workspaces";
import { HostingCause } from "./hosting-cause";
import { LiveVersionSummary } from "./live-version-summary";
import type { SiteContext } from "./site-context";

function Fact({ label, children }: { label: string; children: JSX.Element | string }): JSX.Element {
  return (
    <div className="min-w-0">
      <dt className="text-xs font-medium uppercase tracking-wider text-muted">{label}</dt>
      <dd className="mt-0.5 text-sm">{children}</dd>
    </div>
  );
}

export function SiteLayout(): JSX.Element {
  const { slug = "" } = useParams();
  const site = useSite(slug);
  const me = useMe();
  const workspaces = useWorkspaces();
  const folders = useFolders(site.data?.workspaceSlug);
  usePageTitle(site.data?.displayName ?? "Site");

  if (site.isPending) {
    return <BlockSkeleton label="Loading site" rows={4} />;
  }
  if (site.isError) {
    if (isApiError(site.error) && site.error.status === 404) {
      return <NotFoundPage title="Site not found" message="This site does not exist, or you do not have access to it." />;
    }
    return <LoadError error={site.error} onRetry={() => void site.refetch()} />;
  }

  const data = site.data;
  const isRoot = me.data?.isRoot ?? false;
  const workspace = workspaces.data?.find((item) => item.id === data.workspaceId);
  const folder = data.folderId ? folders.data?.find((item) => item.id === data.folderId) : undefined;
  const base = sitePath(data.slug);
  const context: SiteContext = { site: data, isRoot };

  return (
    <>
      <nav aria-label="Breadcrumb" className="mb-3 text-sm text-muted">
        <Link to="/" className="underline-offset-4 hover:underline">
          Sites
        </Link>
        <span aria-hidden="true"> / </span>
        <Link to={workspacePath(data.workspaceSlug)} className="underline-offset-4 hover:underline">
          {workspace?.name ?? data.workspaceSlug}
        </Link>
      </nav>
      <PageHeader
        title={data.displayName}
        description={<span className="font-mono text-xs">{data.slug}</span>}
        actions={
          <>
            <PublishBadge site={data} />
            <AccessBadge mode={data.accessMode} />
            <AnalyticsBadge enabled={data.analyticsEnabled} />
          </>
        }
      />
      <dl className="mb-4 grid gap-x-8 gap-y-4 rounded-lg border border-line bg-surface p-4 sm:grid-cols-2 lg:grid-cols-4">
        <Fact label="Address">
          <SiteAddress url={data.url} name={data.displayName} />
        </Fact>
        <Fact label="Workspace">{workspace?.name ?? data.workspaceSlug}</Fact>
        <Fact label="Folder">{data.folderId ? (folder?.name ?? "A folder") : "Unfiled"}</Fact>
        <Fact label="Live version">
          <LiveVersionSummary site={data} />
        </Fact>
      </dl>
      <div className="mb-6">
        <HostingNotices site={data} isRoot={isRoot} variant="detail" cause={isRoot ? <HostingCause /> : undefined} />
      </div>
      <TabNav
        label="Site sections"
        items={[
          { to: base, label: "Overview", end: true },
          { to: `${base}/access`, label: "Access" },
          { to: `${base}/domains`, label: "Domains" },
        ]}
      />
      <Outlet context={context} />
    </>
  );
}
