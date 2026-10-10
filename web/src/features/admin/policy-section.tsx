import type { JSX } from "react";
import { Section } from "@/components/ui/section";
import { SettingBytes, SettingNumber, SettingSelect } from "./setting-fields";
import { SettingsForm } from "./settings-form";
import { SettingsGate } from "./settings-gate";

const KEYS = [
  "invites.whoCanInvite",
  "workspaces.whoCanCreate",
  "sites.defaultAccessMode",
  "retention.keepVersions",
  "limits.maxFilesPerVersion",
  "limits.maxVersionBytes",
  "limits.maxFileBytes",
  "limits.maxPendingBytes",
  "limits.maxGateCodeMailsPerHour",
] as const;

export function PolicySection(): JSX.Element {
  return (
    <SettingsGate>
      {(settings) => (
        <div className="flex max-w-3xl flex-col gap-6">
          <Section title="Who can do what" description="The root user can always invite people and create workspaces.">
            <SettingsForm label="Access and policy" settings={settings} keys={KEYS}>
              <SettingSelect
                name="invites.whoCanInvite"
                label="Who can invite new people"
                options={[
                  { value: "root", label: "Only the root user" },
                  { value: "workspaceAdmins", label: "The root user and workspace admins" },
                ]}
                hint="Workspace admins can invite only into workspaces they administer."
              />
              <SettingSelect
                name="workspaces.whoCanCreate"
                label="Who can create shared workspaces"
                options={[
                  { value: "any", label: "Everyone" },
                  { value: "root", label: "Only the root user" },
                ]}
              />
              <SettingSelect
                name="sites.defaultAccessMode"
                label="Access mode of new sites"
                options={[
                  { value: "public", label: "Public" },
                  { value: "restricted", label: "Restricted" },
                ]}
                hint="Existing sites keep their mode. Password mode needs a password per site, so it cannot be the default."
              />
              <h3 className="mt-2 font-display text-lg font-medium">Versions and uploads</h3>
              <SettingNumber name="retention.keepVersions" label="Versions kept per site" min={1} max={10000} hint="Counts the live version. Older finalized versions are deleted after each publish." />
              <SettingNumber name="limits.maxFilesPerVersion" label="Files per version" min={1} max={1000000} />
              <SettingBytes name="limits.maxVersionBytes" label="Size of one version" hint="1 GB is 1024 MB." />
              <SettingBytes name="limits.maxFileBytes" label="Size of one file" hint="Also the largest upload request the server accepts." />
              <SettingBytes name="limits.maxPendingBytes" label="Unfinished uploads in total" allowZero hint="All unfinished versions together. 0 turns the limit off. Each site can hold 3 and each user 10." />
              <h3 className="mt-2 font-display text-lg font-medium">Email sign-in for gated sites</h3>
              <SettingNumber
                name="limits.maxGateCodeMailsPerHour"
                label="Sign-in code emails per hour"
                min={0}
                max={1000000}
                hint="Counts all gated sites together. Past the limit visitors see the usual page and no email is sent. 0 turns the limit off."
              />
            </SettingsForm>
          </Section>
        </div>
      )}
    </SettingsGate>
  );
}
