import type { JSX } from "react";
import { describeError, formatWait } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Section } from "@/components/ui/section";
import { AnalyticsPanel } from "@/features/analytics/analytics-panel";
import type { Settings } from "@/api/types";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { useCheckGeoDatabase } from "@/queries/admin";
import { SettingNumber, SettingSecret, SettingSwitch, SettingText } from "./setting-fields";
import { SettingsForm } from "./settings-form";
import { SettingsGate } from "./settings-gate";

const KEYS = ["analytics.enabledByDefault", "analytics.rawDays", "analytics.retentionDays", "analytics.maxmindAccountId"] as const;
const SECRETS = ["analytics.maxmindLicenseKey"] as const;

const GEO: Record<NonNullable<Settings["analytics.geoStatus"]>, { label: string; tone: "ok" | "danger" | "neutral" }> = {
  unconfigured: { label: "Not set up", tone: "neutral" },
  ok: { label: "Working", tone: "ok" },
  failed: { label: "Last download failed", tone: "danger" },
};

function GeoStatus({ settings }: { settings: Settings }): JSX.Element {
  const status = GEO[settings["analytics.geoStatus"] ?? "unconfigured"];
  const updated = settings["analytics.geoUpdatedAt"];
  const checked = settings["analytics.geoCheckedAt"];
  const detail = settings["analytics.geoDetail"];
  return (
    <dl className="grid gap-3 text-sm sm:grid-cols-2">
      <div>
        <dt className="text-xs font-medium uppercase tracking-wider text-muted">GeoLite2 database</dt>
        <dd className="mt-0.5">
          <Badge tone={status.tone}>{status.label}</Badge>
        </dd>
      </div>
      <div>
        <dt className="text-xs font-medium uppercase tracking-wider text-muted">Last installed</dt>
        <dd className="mt-0.5">{updated ? <time dateTime={updated} title={formatDateTime(updated)}>{formatRelativeTime(updated)}</time> : "Never"}</dd>
      </div>
      <div>
        <dt className="text-xs font-medium uppercase tracking-wider text-muted">Last download attempt</dt>
        <dd className="mt-0.5">{checked ? <time dateTime={checked} title={formatDateTime(checked)}>{formatRelativeTime(checked)}</time> : "Never"}</dd>
      </div>
      {detail ? (
        <div>
          <dt className="text-xs font-medium uppercase tracking-wider text-muted">Note</dt>
          <dd className="mt-0.5 break-words">{detail}</dd>
        </div>
      ) : null}
    </dl>
  );
}

function checkFailureMessage(error: unknown): string {
  const described = describeError(error);
  if (described.kind === "rate_limited") {
    const wait = described.retryAfter === null ? "" : ` Try again in ${formatWait(described.retryAfter)}.`;
    return `The location database can be checked once a minute.${wait}`;
  }
  return described.message;
}

function GeoStatusSection({ settings }: { settings: Settings }): JSX.Element {
  const check = useCheckGeoDatabase();
  const keySaved = settings["analytics.maxmindLicenseKeySet"] === true;
  return (
    <Section
      title="GeoLite2 status"
      description={keySaved ? "The server checks every 5 minutes and downloads a new database each week. Saving a key or account ID checks at once. A new MaxMind key can take a few minutes to become active." : undefined}
      actions={
        keySaved ? (
          <Button variant="secondary" disabled={check.isPending} onClick={() => check.mutate()}>
            {check.isPending ? "Checking…" : "Check now"}
          </Button>
        ) : null
      }
    >
      <GeoStatus settings={settings} />
      <div aria-live="polite" className="empty:hidden">
        {check.isSuccess ? (
          <Alert tone={check.data.status === "failed" ? "warning" : "info"}>
            {check.data.status === "failed" ? `The check failed. ${check.data.detail}` : "The check finished and the location database is working."}
          </Alert>
        ) : null}
        {check.isError ? <Alert tone="error">{checkFailureMessage(check.error)}</Alert> : null}
      </div>
    </Section>
  );
}

export function AnalyticsSection(): JSX.Element {
  return (
    <div className="flex max-w-5xl flex-col gap-6">
      <SettingsGate>
        {(settings) => (
          <div className="flex max-w-3xl flex-col gap-6">
            <Section title="Analytics settings" description="Counting is anonymous: no cookies and no stored addresses.">
              <SettingsForm label="Analytics settings" settings={settings} keys={KEYS} secretKeys={SECRETS}>
                <SettingSwitch name="analytics.enabledByDefault" label="Count page views on new sites" description="Existing sites keep their own setting." />
                <SettingNumber name="analytics.rawDays" label="Days of individual events to keep" min={14} max={365} hint="Hourly charts need the raw events, so the minimum is 14. Older days stay as daily totals." />
                <SettingNumber name="analytics.retentionDays" label="Days of daily totals to keep" hint="Use 0 to keep everything, otherwise 30 to 3650." />
                <h3 className="mt-2 font-display text-lg font-medium">Country and city lookups</h3>
                <p className="text-sm text-muted">With a free MaxMind account the server downloads the GeoLite2 City database and adds countries and cities to reports.</p>
                <SettingText name="analytics.maxmindAccountId" label="MaxMind account ID" hint="Digits only." />
                <SettingSecret name="analytics.maxmindLicenseKey" setFlag="analytics.maxmindLicenseKeySet" label="MaxMind licence key" />
              </SettingsForm>
            </Section>
            <GeoStatusSection settings={settings} />
          </div>
        )}
      </SettingsGate>
      <Section title="Whole instance" description="Page views across every site on this server.">
        <AnalyticsPanel scope={{ kind: "instance" }} isRoot />
      </Section>
    </div>
  );
}
