import { isApiError } from "@/api/client";
import { describeError } from "@/api/errors";
import type { ApiKey } from "@/api/types";

export const MAX_ACTIVE_KEYS = 50;
export const MAX_LABEL_LENGTH = 64;

export function isActive(key: ApiKey): boolean {
  return key.revokedAt === null;
}

export function activeKeyCount(keys: ApiKey[]): number {
  return keys.filter(isActive).length;
}

export function keyStatus(key: ApiKey): { label: string; tone: "ok" | "neutral" } {
  return isActive(key) ? { label: "Active", tone: "ok" } : { label: "Revoked", tone: "neutral" };
}

export function validateLabel(raw: string): string | null {
  const label = raw.trim();
  if (label === "") {
    return "Give the key a label so you can recognise it later.";
  }
  if ([...label].length > MAX_LABEL_LENGTH) {
    return `Use at most ${MAX_LABEL_LENGTH} characters.`;
  }
  return null;
}

export function describeKeyError(error: unknown): string {
  if (isApiError(error) && error.code === "api_key_limit_reached") {
    return `You already have ${MAX_ACTIVE_KEYS} active keys. Revoke one you no longer use, then try again.`;
  }
  return describeError(error).message;
}
