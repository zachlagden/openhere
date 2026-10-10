import type { JSX } from "react";
import type { Version } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type Column } from "@/components/ui/data-table";
import { formatBytes, formatDateTime, formatRelativeTime, pluralise } from "@/lib/format";
import { canDelete, canRestore, describeAuthor, versionLabel } from "@/lib/versions";

interface VersionsTableProps {
  versions: Version[];
  currentUserId: string | undefined;
  editable: boolean;
  onRestore: (version: Version) => void;
  onDelete: (version: Version) => void;
}

export function VersionsTable({ versions, currentUserId, editable, onRestore, onDelete }: VersionsTableProps): JSX.Element {
  const columns: Column<Version>[] = [
    {
      key: "version",
      header: "Version",
      cell: (version) => (
        <span className="inline-flex flex-wrap items-center gap-1.5">
          <code className="font-mono text-[0.8125rem]">{versionLabel(version)}</code>
          {version.live ? <Badge tone="ok">Live</Badge> : null}
          {version.status === "pending" ? <Badge tone="warn">Not finished</Badge> : null}
        </span>
      ),
    },
    {
      key: "created",
      header: "Created",
      cell: (version) => (
        <time dateTime={version.createdAt} title={formatDateTime(version.createdAt)}>
          {formatRelativeTime(version.createdAt)}
        </time>
      ),
    },
    { key: "files", header: "Files", cell: (version) => pluralise(version.fileCount, "file") },
    { key: "size", header: "Size", cell: (version) => formatBytes(version.totalBytes) },
    { key: "by", header: "By", cell: (version) => describeAuthor(version.createdBy, currentUserId) },
  ];
  if (editable) {
    columns.push({
      key: "actions",
      header: "Actions",
      hideLabelOnPhone: true,
      className: "md:text-right",
      cell: (version) => (
        <span className="inline-flex gap-1.5">
          {canRestore(version) ? (
            <Button variant="secondary" size="sm" onClick={() => onRestore(version)} aria-label={`Restore version ${versionLabel(version)}`}>
              Restore
            </Button>
          ) : null}
          {canDelete(version) ? (
            <Button variant="ghost" size="sm" onClick={() => onDelete(version)} aria-label={`Delete version ${versionLabel(version)}`}>
              Delete
            </Button>
          ) : null}
        </span>
      ),
    });
  }
  return <DataTable caption="Versions of this site, newest first" columns={columns} rows={versions} rowKey={(version) => version.id} />;
}
