import { Star, Trash2 } from "lucide-react";
import { useState, type JSX } from "react";
import { describeError } from "@/api/errors";
import type { Domain, DomainVerifyResult } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useToast } from "@/components/ui/toast";
import { challengeRecord, describeCheckResult, domainStatus, targetRecord } from "@/lib/domains";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { useDeleteDomain, useSetPrimaryDomain, useVerifyDomain } from "@/queries/site-detail";
import { DnsRecords } from "./dns-records";

interface DomainCardProps {
  slug: string;
  domain: Domain;
  editable: boolean;
}

export function DomainCard({ slug, domain, editable }: DomainCardProps): JSX.Element {
  const verify = useVerifyDomain(slug);
  const primary = useSetPrimaryDomain(slug);
  const remove = useDeleteDomain(slug);
  const toast = useToast();
  const [result, setResult] = useState<DomainVerifyResult | null>(null);
  const [confirming, setConfirming] = useState(false);
  const status = domainStatus(domain);
  const pending = domain.status !== "verified";

  function runVerify(): void {
    verify.mutate(domain.id, {
      onSuccess: (outcome) => {
        setResult(outcome);
        if (outcome.verified) {
          toast.success(`${domain.domain} is verified.`);
        }
      },
    });
  }

  function togglePrimary(): void {
    primary.mutate(
      { id: domain.id, isPrimary: !domain.isPrimary },
      { onSuccess: () => toast.success(domain.isPrimary ? "Primary domain cleared." : `${domain.domain} is now the primary domain.`) },
    );
  }

  function confirmDelete(): void {
    remove.mutate(domain.id, {
      onSuccess: () => {
        setConfirming(false);
        toast.success(`Removed ${domain.domain}.`);
      },
    });
  }

  const actionError = verify.error ?? primary.error;

  return (
    <li className="flex flex-col gap-3 px-4 py-4 sm:px-5" data-testid="domain-card">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="break-all font-mono text-sm font-medium">{domain.domain}</h3>
        <Badge tone={status.tone}>{status.label}</Badge>
        {domain.isPrimary ? (
          <Badge tone="accent">
            <Star aria-hidden="true" />
            Primary
          </Badge>
        ) : null}
      </div>
      <p className="text-sm text-muted">
        {domain.lastCheckedAt ? (
          <>
            Last checked{" "}
            <time dateTime={domain.lastCheckedAt} title={formatDateTime(domain.lastCheckedAt)}>
              {formatRelativeTime(domain.lastCheckedAt)}
            </time>
            . {describeCheckResult(domain.lastCheckResult)}
          </>
        ) : (
          "Not checked yet."
        )}
      </p>
      {pending ? (
        <>
          <p className="text-sm">Add this TXT record at your DNS provider, then check it. The domain serves nothing until it is verified.</p>
          <DnsRecords rows={[challengeRecord(domain)]} />
        </>
      ) : null}
      <details className="text-sm">
        <summary className="cursor-pointer text-muted hover:text-foreground">How to point {domain.domain} at this server</summary>
        <div className="mt-2 flex flex-col gap-2">
          <DnsRecords rows={[targetRecord(domain)]} />
          <p className="text-muted">{domain.target.note}</p>
        </div>
      </details>
      <div aria-live="polite" className="empty:hidden">
        {result ? <Alert tone={result.verified ? "info" : "warning"}>{result.verified ? "Verified. The TXT record matched." : result.message}</Alert> : null}
        {actionError ? <Alert tone="error">{describeError(actionError).message}</Alert> : null}
      </div>
      {editable ? (
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" disabled={verify.isPending} onClick={runVerify}>
            {verify.isPending ? "Checking…" : pending ? "Check DNS now" : "Re-check DNS"}
          </Button>
          {!pending ? (
            <Button variant="secondary" size="sm" disabled={primary.isPending} onClick={togglePrimary}>
              {domain.isPrimary ? "Clear primary" : "Make primary"}
            </Button>
          ) : null}
          <Button variant="ghost" size="sm" onClick={() => setConfirming(true)}>
            <Trash2 /> Delete
          </Button>
        </div>
      ) : null}
      <ConfirmDialog
        open={confirming}
        onOpenChange={(open) => {
          if (!open) {
            remove.reset();
          }
          setConfirming(open);
        }}
        title={`Delete ${domain.domain}?`}
        confirmLabel="Delete domain"
        pending={remove.isPending}
        error={remove.error ? describeError(remove.error).message : null}
        onConfirm={confirmDelete}
      >
        <p>The domain stops serving this site at once and its name becomes free for any site to claim. You can add it again later.</p>
      </ConfirmDialog>
    </li>
  );
}
