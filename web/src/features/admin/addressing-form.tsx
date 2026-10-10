import type { JSX } from "react";
import type { Settings } from "@/api/types";
import { Section } from "@/components/ui/section";
import { SettingSwitch, SettingText } from "./setting-fields";
import { SettingsForm } from "./settings-form";

const KEYS = ["hosting.baseDomain", "hosting.pathEnabled", "hosting.subdomainEnabled"] as const;

export function AddressingForm({ settings }: { settings: Settings }): JSX.Element {
  const subdomainOn = settings["hosting.subdomainEnabled"] === true;
  const wildcardOk = settings["hosting.wildcardStatus"] === "ok";
  return (
    <Section title="Site addresses" description="Where the server serves published sites.">
      <SettingsForm label="Site addresses" settings={settings} keys={KEYS}>
        <SettingText
          name="hosting.baseDomain"
          label="Base domain"
          placeholder="example.com"
          hint="Sites get {slug}.{base domain}. Leave it empty to use the host name of the server address. Changing it resets the wildcard check."
        />
        <SettingSwitch
          name="hosting.subdomainEnabled"
          label="Subdomain addresses"
          disabled={!subdomainOn && !wildcardOk}
          description={
            <>
              Serve each site at its own subdomain. This is the safer mode, because every site gets its own origin. It needs wildcard DNS and, over HTTPS, a wildcard certificate.
              {!subdomainOn && !wildcardOk ? " Pass the wildcard check below first." : ""}
            </>
          }
        />
        <SettingSwitch
          name="hosting.pathEnabled"
          label="Path addresses"
          description="Serve each site under /s/{slug}/ on the server address. Pages run in a sandbox and cannot use cookies or storage. Gated sites need HTTPS in this mode."
        />
        <p className="text-sm text-muted">At least one of the two must stay on. The server refuses a save that turns both off.</p>
      </SettingsForm>
    </Section>
  );
}
