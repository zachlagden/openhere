import type { JSX } from "react";
import type { Site } from "@/api/types";
import { formatBytes, formatDateTime, formatRelativeTime, pluralise } from "@/lib/format";
import { useVersions } from "@/queries/site-detail";

export function LiveVersionSummary({ site }: { site: Site }): JSX.Element {
  const versions = useVersions(site.slug);
  if (site.currentVersionId === null) {
    return <span>Nothing published yet. Your agent publishes through the API.</span>;
  }
  const live = versions.data?.find((version) => version.live);
  if (!live) {
    return <span>{versions.isPending ? "Loading version…" : "A version is live."}</span>;
  }
  const published = live.finalizedAt ?? live.createdAt;
  return (
    <span>
      {pluralise(live.fileCount, "file")}, {formatBytes(live.totalBytes)}, published{" "}
      <time dateTime={published} title={formatDateTime(published)}>
        {formatRelativeTime(published)}
      </time>
    </span>
  );
}
