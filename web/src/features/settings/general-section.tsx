import type { JSX } from "react";
import { Section } from "@/components/ui/section";
import { CopyField } from "@/components/ui/copy-field";
import { cn } from "@/lib/cn";
import { useTheme, type ThemePreference } from "@/lib/theme";
import { useInstance } from "@/queries/instance";

const THEMES: { value: ThemePreference; label: string; hint: string }[] = [
  { value: "system", label: "System", hint: "Follow your device setting." },
  { value: "light", label: "Light", hint: "Warm paper, dark ink." },
  { value: "dark", label: "Dark", hint: "Dark paper, light ink." },
];

export function GeneralSection(): JSX.Element {
  const { preference, setPreference } = useTheme();
  const instance = useInstance();
  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <Section title="Appearance" description="Saved in this browser only.">
        <fieldset className="grid gap-2 sm:grid-cols-3">
          <legend className="sr-only">Theme</legend>
          {THEMES.map((theme) => (
            <label
              key={theme.value}
              className={cn(
                "flex cursor-pointer items-start gap-2.5 rounded-md border px-3 py-2.5",
                preference === theme.value ? "border-accent bg-accent-soft" : "border-line hover:bg-sunken",
              )}
            >
              <input type="radio" name="theme" value={theme.value} checked={preference === theme.value} onChange={() => setPreference(theme.value)} className="mt-1 accent-[var(--accent)]" />
              <span>
                <span className="block text-sm font-medium">{theme.label}</span>
                <span className="block text-xs text-muted">{theme.hint}</span>
              </span>
            </label>
          ))}
        </fieldset>
      </Section>
      <Section title="This server" description="Facts about the server you are signed in to.">
        {instance.data ? (
          <div className="flex flex-col gap-3">
            <CopyField label="Server address" value={instance.data.publicUrl} />
            <CopyField label="Agent credentials file" value={instance.data.agentCredentialsPath} />
            <dl className="grid gap-3 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-xs font-medium uppercase tracking-wider text-muted">Version</dt>
                <dd className="mt-0.5 font-mono text-[0.8125rem]">{instance.data.version}</dd>
              </div>
              <div>
                <dt className="text-xs font-medium uppercase tracking-wider text-muted">Email</dt>
                <dd className="mt-0.5">{instance.data.emailAvailable ? "The server can send email." : "The server cannot send email."}</dd>
              </div>
            </dl>
          </div>
        ) : (
          <p className="text-sm text-muted">Loading…</p>
        )}
      </Section>
    </div>
  );
}
