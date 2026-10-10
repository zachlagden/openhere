export const MAX_ALLOWLIST_ENTRIES = 500;

export type AllowlistKind = "email" | "domain";

export type EntryResult = { ok: true; value: string } | { ok: false; message: string };

const LABEL = "[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?";
const DOMAIN_PATTERN = new RegExp(`^(${LABEL}\\.)+${LABEL}$`);
const EMAIL_PATTERN = new RegExp(`^[a-z0-9.!#$%&'*+/=?^_\`{|}~-]{1,64}@(${LABEL}\\.)+${LABEL}$`);
const MAX_EMAIL_LENGTH = 254;
const MAX_DOMAIN_LENGTH = 253;

function isAscii(value: string): boolean {
  for (const char of value) {
    if ((char.codePointAt(0) ?? 0) > 0x7f) {
      return false;
    }
  }
  return true;
}

export function normaliseEmail(raw: string): EntryResult {
  const value = raw.trim().toLowerCase();
  if (value === "") {
    return { ok: false, message: "Enter an email address." };
  }
  if (!isAscii(value) || value.length > MAX_EMAIL_LENGTH || !EMAIL_PATTERN.test(value)) {
    return { ok: false, message: "Enter a plain email address such as sam@example.com." };
  }
  return { ok: true, value };
}

export function normaliseDomain(raw: string): EntryResult {
  const value = raw.trim().toLowerCase().replace(/^@/, "");
  if (value === "") {
    return { ok: false, message: "Enter a domain." };
  }
  if (!isAscii(value) || value.length > MAX_DOMAIN_LENGTH || !DOMAIN_PATTERN.test(value)) {
    return { ok: false, message: "Enter a bare domain such as example.com, without a scheme, path or wildcard." };
  }
  return { ok: true, value };
}

export function normaliseEntry(kind: AllowlistKind, raw: string): EntryResult {
  return kind === "email" ? normaliseEmail(raw) : normaliseDomain(raw);
}

export type AddResult = { ok: true; list: string[] } | { ok: false; message: string };

export function addEntry(list: string[], kind: AllowlistKind, raw: string): AddResult {
  const entry = normaliseEntry(kind, raw);
  if (!entry.ok) {
    return entry;
  }
  if (list.includes(entry.value)) {
    return { ok: false, message: `${entry.value} is already on the list.` };
  }
  if (list.length >= MAX_ALLOWLIST_ENTRIES) {
    return { ok: false, message: `A list holds at most ${MAX_ALLOWLIST_ENTRIES} entries.` };
  }
  return { ok: true, list: [...list, entry.value].sort() };
}

export function removeEntry(list: string[], value: string): string[] {
  return list.filter((item) => item !== value);
}
