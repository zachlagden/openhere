import type { JSX } from "react";
import { describeError } from "@/api/errors";
import type { Settings } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Section } from "@/components/ui/section";
import { SettingNumber, SettingSecret, SettingSelect, SettingText } from "./setting-fields";
import { SettingsForm, useSettingsFormApi } from "./settings-form";
import { SettingsGate } from "./settings-gate";
import { useSendTestEmail } from "@/queries/admin";

const KEYS = ["email.provider", "email.fromAddress", "email.fromName", "email.smtp.host", "email.smtp.port", "email.smtp.security", "email.smtp.username"] as const;
const SECRETS = ["email.smtp.password", "email.resend.apiKey"] as const;

function ProviderFields(): JSX.Element {
  const { draft } = useSettingsFormApi();
  const provider = draft["email.provider"];
  return (
    <>
      <SettingSelect
        name="email.provider"
        label="Provider"
        options={[
          { value: "none", label: "None (email is off)" },
          { value: "smtp", label: "SMTP server" },
          { value: "resend", label: "Resend" },
        ]}
        hint="Email sends invites, password reset links and sign-in codes for restricted sites."
      />
      {provider === "none" ? null : (
        <div className="grid gap-4 sm:grid-cols-2">
          <SettingText name="email.fromAddress" label="From address" type="email" placeholder="noreply@example.com" />
          <SettingText name="email.fromName" label="From name" placeholder="Example Sites" hint="Optional." />
        </div>
      )}
      {provider === "smtp" ? (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <SettingText name="email.smtp.host" label="SMTP host" placeholder="smtp.example.com" />
            <SettingNumber name="email.smtp.port" label="SMTP port" min={1} max={65535} />
            <SettingSelect
              name="email.smtp.security"
              label="Connection security"
              options={[
                { value: "starttls", label: "STARTTLS (usually port 587)" },
                { value: "tls", label: "TLS (usually port 465)" },
                { value: "none", label: "None" },
              ]}
            />
            <SettingText name="email.smtp.username" label="Username" hint="Leave empty for a server without authentication." />
          </div>
          <SettingSecret name="email.smtp.password" setFlag="email.smtp.passwordSet" label="SMTP password" />
        </>
      ) : null}
      {provider === "resend" ? <SettingSecret name="email.resend.apiKey" setFlag="email.resend.apiKeySet" label="Resend API key" /> : null}
    </>
  );
}

function TestEmail(): JSX.Element {
  const test = useSendTestEmail();
  return (
    <Section title="Send a test email" description="Sends a short message to your own address through the saved settings. Save any changes first.">
      <div>
        <Button variant="secondary" disabled={test.isPending} onClick={() => test.mutate()}>
          {test.isPending ? "Sending…" : "Send test email"}
        </Button>
      </div>
      <div aria-live="polite" className="empty:hidden">
        {test.isSuccess ? <Alert tone="info">A test email was handed to the provider for {test.data.to}.</Alert> : null}
        {test.isError ? <Alert tone="error">{describeError(test.error).message}</Alert> : null}
      </div>
    </Section>
  );
}

export function EmailSection(): JSX.Element {
  return (
    <SettingsGate>
      {(settings: Settings) => (
        <div className="flex max-w-3xl flex-col gap-6">
          <Section title="Email" description="Passwords and keys are encrypted at rest and never shown again.">
            <SettingsForm label="Email settings" settings={settings} keys={KEYS} secretKeys={SECRETS}>
              <ProviderFields />
            </SettingsForm>
          </Section>
          <TestEmail />
        </div>
      )}
    </SettingsGate>
  );
}
