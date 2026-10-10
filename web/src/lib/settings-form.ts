import { isApiError } from "@/api/client";
import { describeError } from "@/api/errors";
import type { Settings, SettingsPatch } from "@/api/types";
import { changedFields } from "./diff";

export type SettingKey = keyof Settings & string;
export type SettingsDraft = SettingsPatch;

export function pickSettings(settings: Settings, keys: readonly SettingKey[]): SettingsDraft {
  const picked: Record<string, unknown> = {};
  for (const key of keys) {
    if (key in settings) {
      picked[key] = settings[key];
    }
  }
  return picked as SettingsDraft;
}

export function buildSettingsPatch(original: SettingsDraft, draft: SettingsDraft, secrets: Partial<Record<SettingKey, string>> = {}): SettingsDraft {
  const patch: Record<string, unknown> = { ...changedFields(original, draft) };
  for (const [key, value] of Object.entries(secrets)) {
    if (typeof value === "string" && value !== "") {
      patch[key] = value;
    }
  }
  for (const [key, value] of Object.entries(patch)) {
    if (typeof value === "number" && !Number.isFinite(value)) {
      delete patch[key];
    }
  }
  return patch as SettingsDraft;
}

export function fieldFromMessage(message: string, keys: readonly string[]): string | null {
  let best: string | null = null;
  for (const key of keys) {
    if (message.includes(key) && (best === null || key.length > best.length)) {
      best = key;
    }
  }
  return best;
}

const CODE_MESSAGES: Record<string, string> = {
  wildcard_not_verified: "Subdomain addresses can only be switched on after a wildcard check passes. Run the check first.",
  no_hosting_mode: "At least one of the subdomain and path addresses must stay on, or no site could be reached.",
  base_domain_in_use: "A custom domain already uses that base domain or a name under it. Delete that domain first.",
  email_not_configured: "Email is not set up yet. Save a working configuration first, then send the test.",
};

export interface SettingsFailure {
  field: string | null;
  message: string;
}

export function describeSettingsFailure(error: unknown, keys: readonly string[]): SettingsFailure {
  if (isApiError(error)) {
    const known = CODE_MESSAGES[error.code];
    if (known) {
      return { field: null, message: known };
    }
    if (error.code === "validation_failed") {
      return { field: fieldFromMessage(error.message, keys), message: error.message };
    }
  }
  return { field: null, message: describeError(error).message };
}

export function fieldError(failure: SettingsFailure | null, key: string): string | null {
  return failure !== null && failure.field === key ? failure.message : null;
}

export function formError(failure: SettingsFailure | null): string | null {
  return failure !== null && failure.field === null ? failure.message : null;
}

export function destructiveChanges(original: SettingsDraft, patch: SettingsDraft): string[] {
  const reasons: string[] = [];
  if (patch["hosting.subdomainEnabled"] === false && original["hosting.subdomainEnabled"] === true) {
    reasons.push("Subdomain addresses stop working. Sites that rely on them become unreachable there.");
  }
  if (patch["hosting.pathEnabled"] === false && original["hosting.pathEnabled"] === true) {
    reasons.push("Path addresses stop working. Sites that rely on them become unreachable there.");
  }
  if (patch["hosting.baseDomain"] !== undefined) {
    reasons.push("Changing the base domain resets the wildcard check and switches subdomain addresses off until a new check passes.");
  }
  if (patch["hosting.customDomainsEnabled"] === false && original["hosting.customDomainsEnabled"] === true) {
    reasons.push("Every custom domain stops serving its site and answers like an unknown host.");
  }
  if (patch["email.provider"] === "none" && original["email.provider"] !== "none") {
    reasons.push("Invite emails, reset emails and sign-in codes stop working.");
  }
  if (patch["analytics.retentionDays"] !== undefined && typeof patch["analytics.retentionDays"] === "number" && patch["analytics.retentionDays"] !== 0) {
    const before = original["analytics.retentionDays"];
    if (before === 0 || (typeof before === "number" && patch["analytics.retentionDays"] < before)) {
      reasons.push("Analytics older than the new limit is deleted the next time maintenance runs.");
    }
  }
  return reasons;
}

export function buildClearPatch(key: SettingKey): SettingsPatch {
  return Object.fromEntries([[key, ""]]) as SettingsPatch;
}
