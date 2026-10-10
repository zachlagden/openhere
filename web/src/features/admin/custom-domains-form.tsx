import type { JSX } from "react";
import type { Settings } from "@/api/types";
import { Section } from "@/components/ui/section";
import { SettingNumber, SettingSwitch } from "./setting-fields";
import { SettingsForm } from "./settings-form";

const KEYS = ["hosting.customDomainsEnabled", "hosting.maxDomainsPerSite"] as const;

export function CustomDomainsForm({ settings }: { settings: Settings }): JSX.Element {
  return (
    <Section title="Custom domains" description="Let people serve their sites at their own domain names, once they prove control with a DNS record.">
      <SettingsForm label="Custom domains" settings={settings} keys={KEYS}>
        <SettingSwitch
          name="hosting.customDomainsEnabled"
          label="Serve custom domains"
          description="When off, every custom domain answers like an unknown host. Stored domains stay as they are."
        />
        <SettingNumber name="hosting.maxDomainsPerSite" label="Domains per site" min={1} max={50} hint="Pending and verified domains count. Lowering it keeps domains a site already has." />
      </SettingsForm>
    </Section>
  );
}
