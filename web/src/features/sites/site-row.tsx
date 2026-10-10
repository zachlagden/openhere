import { Folder as FolderIcon, Layers } from "lucide-react";
import type { JSX } from "react";
import { Link } from "react-router";
import type { Site } from "@/api/types";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { HostingNotices } from "./hosting-notices";
import { SiteAddress } from "./site-address";
import { AccessBadge, AnalyticsBadge, PublishBadge } from "./site-badges";
import { SiteMenu } from "./site-menu";

interface SiteRowProps {
  site: Site;
  workspaceName: string;
  folderName: string | null;
  isRoot: boolean;
  now: Date;
}

export function SiteRow({ site, workspaceName, folderName, isRoot, now }: SiteRowProps): JSX.Element {
  const showSlug = site.displayName !== site.slug;
  return (
    <li className="flex flex-col gap-3 px-4 py-4 sm:px-5" data-testid="site-row">
      <div className="grid gap-x-6 gap-y-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
        <div className="min-w-0">
          <div className="flex items-start gap-2">
            <h3 className="min-w-0 break-words font-medium leading-snug">
              <Link to={`/sites/${encodeURIComponent(site.slug)}`} className="underline-offset-4 hover:underline">
                {site.displayName}
              </Link>
            </h3>
            <div className="ml-auto md:hidden">
              <SiteMenu site={site} />
            </div>
          </div>
          {showSlug ? <p className="truncate font-mono text-xs text-muted">{site.slug}</p> : null}
          <p className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
            <span className="inline-flex items-center gap-1">
              <Layers aria-hidden="true" className="size-3.5" />
              {workspaceName}
            </span>
            <span className="inline-flex items-center gap-1">
              <FolderIcon aria-hidden="true" className="size-3.5" />
              {folderName ?? "Unfiled"}
            </span>
            <time dateTime={site.updatedAt} title={formatDateTime(site.updatedAt)}>
              Updated {formatRelativeTime(site.updatedAt, now)}
            </time>
          </p>
        </div>
        <div className="flex min-w-0 flex-col gap-2.5">
          <div className="flex items-center gap-1">
            <div className="min-w-0 flex-1">
              <SiteAddress url={site.url} name={site.displayName} />
            </div>
            <div className="hidden md:block">
              <SiteMenu site={site} />
            </div>
          </div>
          <div className="flex flex-wrap gap-1.5">
            <PublishBadge site={site} />
            <AccessBadge mode={site.accessMode} />
            <AnalyticsBadge enabled={site.analyticsEnabled} />
          </div>
        </div>
      </div>
      <HostingNotices site={site} isRoot={isRoot} variant="list" />
    </li>
  );
}
