import type { JSX } from "react";
import type { Settings } from "@/api/types";
import { Section } from "@/components/ui/section";
import { SettingSecret, SettingSelect } from "./setting-fields";
import { SettingsForm } from "./settings-form";

const KEYS = ["tls.dnsProvider"] as const;
const SECRETS = ["tls.cloudflareApiToken"] as const;

export function DnsProviderForm({ settings }: { settings: Settings }): JSX.Element {
  return (
    <Section title="DNS provider for certificates" description="Lets the server prove control of the base domain to get a wildcard certificate. It applies only when the server runs with built-in HTTPS.">
      <SettingsForm label="DNS provider" settings={settings} keys={KEYS} secretKeys={SECRETS}>
        <SettingSelect
          name="tls.dnsProvider"
          label="Provider"
          options={[
            { value: "none", label: "None" },
            { value: "cloudflare", label: "Cloudflare" },
          ]}
          hint="With none, no wildcard certificate is requested."
        />
        <SettingSecret
          name="tls.cloudflareApiToken"
          setFlag="tls.cloudflareApiTokenSet"
          label="Cloudflare API token"
          hint="Needs Zone:DNS:Edit and Zone:Zone:Read on the zone of the base domain. Write-only."
        />
      </SettingsForm>
    </Section>
  );
}
