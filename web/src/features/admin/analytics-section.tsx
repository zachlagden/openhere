import type { JSX } from "react";
import { Badge } from "@/components/ui/badge";
import { Section } from "@/components/ui/section";
import { AnalyticsPanel } from "@/features/analytics/analytics-panel";
import type { Settings } from "@/api/types";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
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
            <Section title="GeoLite2 status">
              <GeoStatus settings={settings} />
            </Section>
          </div>
        )}
      </SettingsGate>
      <Section title="Whole instance" description="Page views across every site on this server.">
        <AnalyticsPanel scope={{ kind: "instance" }} isRoot />
      </Section>
    </div>
  );
}
