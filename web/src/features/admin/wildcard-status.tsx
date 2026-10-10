import type { JSX } from "react";
import type { Settings } from "@/api/types";
import { describeError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Section } from "@/components/ui/section";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { useCheckWildcard } from "@/queries/admin";

const STATUS: Record<Settings["hosting.wildcardStatus"] & string, { label: string; tone: "ok" | "danger" | "neutral" }> = {
  unchecked: { label: "Not checked", tone: "neutral" },
  ok: { label: "Passing", tone: "ok" },
  failed: { label: "Failing", tone: "danger" },
};

export function WildcardStatus({ settings }: { settings: Settings }): JSX.Element {
  const check = useCheckWildcard();
  const status = STATUS[settings["hosting.wildcardStatus"] ?? "unchecked"];
  const checkedAt = settings["hosting.wildcardCheckedAt"];
  const detail = settings["hosting.wildcardDetail"];
  const failures = settings["hosting.wildcardFailures"] ?? 0;
  const disabled = settings["hosting.subdomainDisabledReason"] === "wildcard_check_failed";
  return (
    <Section
      title="Wildcard check"
      description="Subdomain addresses need wildcard DNS that reaches this server. The check requests a random subdomain and expects this server to answer. It runs at startup and every 6 hours."
      actions={
        <Button variant="secondary" disabled={check.isPending} onClick={() => check.mutate()}>
          {check.isPending ? "Checking…" : "Check now"}
        </Button>
      }
    >
      <dl className="grid gap-3 text-sm sm:grid-cols-2">
        <div>
          <dt className="text-xs font-medium uppercase tracking-wider text-muted">Status</dt>
          <dd className="mt-0.5">
            <Badge tone={status.tone}>{status.label}</Badge>
          </dd>
        </div>
        <div>
          <dt className="text-xs font-medium uppercase tracking-wider text-muted">Last check</dt>
          <dd className="mt-0.5">
            {checkedAt ? (
              <time dateTime={checkedAt} title={formatDateTime(checkedAt)}>
                {formatRelativeTime(checkedAt)}
              </time>
            ) : (
              "Never"
            )}
          </dd>
        </div>
        <div>
          <dt className="text-xs font-medium uppercase tracking-wider text-muted">Failed checks in a row</dt>
          <dd className="mt-0.5">{failures}</dd>
        </div>
        {detail ? (
          <div>
            <dt className="text-xs font-medium uppercase tracking-wider text-muted">Detail</dt>
            <dd className="mt-0.5 break-words">{detail}</dd>
          </div>
        ) : null}
      </dl>
      {disabled ? <Alert tone="warning">Subdomain addresses were switched off automatically after 3 failed checks in a row. Fix the DNS, run the check, then switch them back on.</Alert> : null}
      <div aria-live="polite" className="empty:hidden">
        {check.isSuccess ? (
          <Alert tone={check.data.wildcard.status === "ok" ? "info" : "warning"}>
            {check.data.wildcard.status === "ok" ? "The check passed. Subdomain addresses can be switched on." : `The check failed. ${check.data.wildcard.detail}`}
          </Alert>
        ) : null}
        {check.isError ? <Alert tone="error">{describeError(check.error).message}</Alert> : null}
      </div>
    </Section>
  );
}
