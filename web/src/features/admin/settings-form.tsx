import { createContext, Fragment, use, useMemo, useState, type FormEvent, type JSX, type ReactNode } from "react";
import type { Notice, Settings } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { SaveBar } from "@/components/ui/save-bar";
import { useToast } from "@/components/ui/toast";
import {
  buildSettingsPatch,
  describeSettingsFailure,
  destructiveChanges,
  formError,
  pickSettings,
  type SettingKey,
  type SettingsDraft,
  type SettingsFailure,
} from "@/lib/settings-form";
import { useUpdateSettings } from "@/queries/admin";

export interface SettingsFormApi {
  settings: Settings;
  draft: SettingsDraft;
  secrets: Partial<Record<SettingKey, string>>;
  failure: SettingsFailure | null;
  set: (key: SettingKey, value: unknown) => void;
  setSecret: (key: SettingKey, value: string) => void;
}

const SettingsFormContext = createContext<SettingsFormApi | null>(null);

export function useSettingsFormApi(): SettingsFormApi {
  const api = use(SettingsFormContext);
  if (!api) {
    throw new Error("Setting fields must be rendered inside a SettingsForm.");
  }
  return api;
}

interface SettingsFormProps {
  label: string;
  settings: Settings;
  keys: readonly SettingKey[];
  secretKeys?: readonly SettingKey[];
  saveLabel?: string;
  children: ReactNode;
}

export function SettingsForm({ label, settings, keys, secretKeys = [], saveLabel, children }: SettingsFormProps): JSX.Element {
  const update = useUpdateSettings();
  const toast = useToast();
  const original = useMemo(() => pickSettings(settings, keys), [settings, keys]);
  const [draft, setDraft] = useState<SettingsDraft>(original);
  const [secrets, setSecrets] = useState<Partial<Record<SettingKey, string>>>({});
  const [failure, setFailure] = useState<SettingsFailure | null>(null);
  const [notices, setNotices] = useState<Notice[]>([]);
  const [confirming, setConfirming] = useState<string[] | null>(null);
  const [resets, setResets] = useState(0);
  const patch = buildSettingsPatch(original, draft, secrets);
  const dirty = Object.keys(patch).length > 0;
  const allKeys = [...keys, ...secretKeys];

  function save(): void {
    setFailure(null);
    update.mutate(patch, {
      onSuccess: (result) => {
        setConfirming(null);
        setNotices(result.notices);
        setSecrets({});
        setDraft(pickSettings(result.settings, keys));
        setResets((count) => count + 1);
        toast.success("Settings saved.");
      },
      onError: (error) => {
        setConfirming(null);
        setFailure(describeSettingsFailure(error, allKeys));
      },
    });
  }

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const reasons = destructiveChanges(original, patch);
    if (reasons.length > 0) {
      setConfirming(reasons);
      return;
    }
    save();
  }

  const api = useMemo<SettingsFormApi>(
    () => ({
      settings,
      draft,
      secrets,
      failure,
      set: (key, value) => setDraft((current) => ({ ...current, [key]: value })),
      setSecret: (key, value) => setSecrets((current) => ({ ...current, [key]: value })),
    }),
    [settings, draft, secrets, failure],
  );

  const message = formError(failure);

  return (
    <SettingsFormContext value={api}>
      <form onSubmit={submit} aria-label={label} noValidate className="flex flex-col gap-4">
        <Fragment key={resets}>{children}</Fragment>
        {message ? <Alert tone="error">{message}</Alert> : null}
        <div aria-live="polite" className="empty:hidden">
          {notices.map((notice) => (
            <Alert key={notice.code} tone="info">
              {notice.message}
            </Alert>
          ))}
        </div>
        <SaveBar
          dirty={dirty}
          pending={update.isPending}
          saveLabel={saveLabel}
          onDiscard={() => {
            setDraft(original);
            setSecrets({});
            setFailure(null);
            setResets((count) => count + 1);
          }}
        />
      </form>
      <ConfirmDialog
        open={confirming !== null}
        onOpenChange={(open) => {
          if (!open) {
            setConfirming(null);
          }
        }}
        title="Save these changes?"
        confirmLabel="Save changes"
        pending={update.isPending}
        error={null}
        onConfirm={save}
      >
        <ul className="list-disc pl-5">
          {(confirming ?? []).map((reason) => (
            <li key={reason}>{reason}</li>
          ))}
        </ul>
      </ConfirmDialog>
    </SettingsFormContext>
  );
}
