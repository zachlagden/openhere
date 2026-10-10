import type { JSX } from "react";
import type { HostingDetail } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Section } from "@/components/ui/section";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { useHostingStatus } from "@/queries/admin";

function When({ iso }: { iso: string | null }): JSX.Element {
  if (!iso) {
    return <span className="text-muted">Not available</span>;
  }
  return (
    <time dateTime={iso} title={formatDateTime(iso)}>
      {formatDateTime(iso)} ({formatRelativeTime(iso)})
    </time>
  );
}

const WILDCARD_TONE = { none: "neutral", pending: "warn", ok: "ok", failed: "danger" } as const;

function Facts({ tls }: { tls: HostingDetail["tls"] }): JSX.Element {
  if (!tls.enabled) {
    return (
      <p className="text-sm text-muted">
        Built-in HTTPS is off, so the server does not manage certificates. A reverse proxy in front of it should handle HTTPS.
        {tls.dnsProviderConfigured ? " A DNS provider is already stored for when you turn it on." : ""}
      </p>
    );
  }
  return (
    <dl className="grid gap-3 text-sm sm:grid-cols-2">
      <div>
        <dt className="text-xs font-medium uppercase tracking-wider text-muted">Server address certificate expires</dt>
        <dd className="mt-0.5">
          <When iso={tls.managementCertificateExpiresAt} />
        </dd>
      </div>
      <div>
        <dt className="text-xs font-medium uppercase tracking-wider text-muted">Wildcard certificate</dt>
        <dd className="mt-0.5">
          <Badge tone={WILDCARD_TONE[tls.wildcard.state]}>{tls.wildcard.state}</Badge>
          {tls.wildcard.reason ? <span className="ml-2 text-muted">{tls.wildcard.reason}</span> : null}
        </dd>
      </div>
      <div>
        <dt className="text-xs font-medium uppercase tracking-wider text-muted">Custom domain certificates</dt>
        <dd className="mt-0.5">{tls.customDomains.count} loaded</dd>
      </div>
      <div>
        <dt className="text-xs font-medium uppercase tracking-wider text-muted">Soonest custom domain expiry</dt>
        <dd className="mt-0.5">
          <When iso={tls.customDomains.soonestExpiresAt} />
        </dd>
      </div>
      <div>
        <dt className="text-xs font-medium uppercase tracking-wider text-muted">DNS provider</dt>
        <dd className="mt-0.5">{tls.dnsProviderConfigured ? "Configured" : "Not configured"}</dd>
      </div>
    </dl>
  );
}

export function TlsStatus(): JSX.Element {
  const status = useHostingStatus();
  return (
    <Section title="Certificates" description="Only the root user sees this. It reads the live state of the built-in HTTPS support.">
      {status.isPending ? <BlockSkeleton label="Loading certificate state" rows={2} /> : null}
      {status.isError ? <LoadError error={status.error} onRetry={() => void status.refetch()} /> : null}
      {status.data?.detail ? <Facts tls={status.data.detail.tls} /> : null}
    </Section>
  );
}
