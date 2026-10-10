import type { JSX } from "react";
import { AddressingForm } from "./addressing-form";
import { CustomDomainsForm } from "./custom-domains-form";
import { DnsProviderForm } from "./dns-provider-form";
import { SettingsGate } from "./settings-gate";
import { TlsStatus } from "./tls-status";
import { WildcardStatus } from "./wildcard-status";

export function HostingSection(): JSX.Element {
  return (
    <SettingsGate>
      {(settings) => (
        <div className="flex max-w-3xl flex-col gap-6">
          <AddressingForm settings={settings} />
          <WildcardStatus settings={settings} />
          <CustomDomainsForm settings={settings} />
          <TlsStatus />
          <DnsProviderForm settings={settings} />
        </div>
      )}
    </SettingsGate>
  );
}
