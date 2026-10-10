import { Globe } from "lucide-react";
import type { JSX } from "react";
import { Link } from "react-router";
import { sitePath } from "@/api/paths";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { DataTable, type Column } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import type { DomainRow } from "@/lib/domains-overview";
import { countWaiting } from "@/lib/domains-overview";
import { describeCheckResult, domainStatus } from "@/lib/domains";
import { formatRelativeTime } from "@/lib/format";
import { useDomainsOverview } from "@/queries/domains-overview";
import { useInstance } from "@/queries/instance";

const COLUMNS: Column<DomainRow>[] = [
  { key: "domain", header: "Domain", cell: (row) => <span className="break-all font-mono text-[0.8125rem]">{row.domain.domain}</span> },
  {
    key: "site",
    header: "Site",
    cell: (row) => (
      <Link to={`${sitePath(row.site.slug)}/domains`} className="underline underline-offset-4">
        {row.site.displayName}
      </Link>
    ),
  },
  {
    key: "status",
    header: "Status",
    cell: (row) => {
      const status = domainStatus(row.domain);
      return (
        <span className="inline-flex flex-wrap gap-1.5">
          <Badge tone={status.tone}>{status.label}</Badge>
          {row.domain.isPrimary ? <Badge tone="accent">Primary</Badge> : null}
        </span>
      );
    },
  },
  {
    key: "checked",
    header: "Last check",
    cell: (row) => (row.domain.lastCheckedAt ? `${formatRelativeTime(row.domain.lastCheckedAt)}. ${describeCheckResult(row.domain.lastCheckResult)}` : "Not checked yet."),
  },
];

export function DomainsOverviewSection(): JSX.Element {
  const overview = useDomainsOverview();
  const instance = useInstance();
  const waiting = overview.data ? countWaiting(overview.data.rows) : 0;
  return (
    <div className="flex max-w-5xl flex-col gap-6">
      {instance.data && !instance.data.hostingModes.customDomains ? (
        <Alert tone="warning">Custom domains are switched off for this server. Domains stay saved but serve nothing until an administrator turns the feature on.</Alert>
      ) : null}
      <Section title="Custom domains" description="Every custom domain on the sites you can reach. Open a site to add, verify or remove domains.">
        {overview.isPending ? <BlockSkeleton label="Loading domains" rows={3} /> : null}
        {overview.isError ? <LoadError error={overview.error} onRetry={() => void overview.refetch()} /> : null}
        {overview.data && overview.data.rows.length === 0 ? (
          <EmptyState icon={<Globe />} title="No custom domains">
            <p>Add a custom domain from the Domains tab of any site.</p>
          </EmptyState>
        ) : null}
        {overview.data && overview.data.rows.length > 0 ? (
          <>
            <p aria-live="polite" className="text-sm text-muted">
              {overview.data.rows.length} {overview.data.rows.length === 1 ? "domain" : "domains"}
              {waiting > 0 ? `, ${waiting} waiting for DNS` : ""}.
            </p>
            <DataTable caption="Custom domains across your sites" columns={COLUMNS} rows={overview.data.rows} rowKey={(row) => row.domain.id} />
          </>
        ) : null}
        {overview.data && overview.data.unreadable > 0 ? <Alert tone="warning">The domains of {overview.data.unreadable} sites could not be loaded.</Alert> : null}
        {overview.data?.truncated ? <Alert tone="info">Only the most recent sites are included.</Alert> : null}
      </Section>
    </div>
  );
}
