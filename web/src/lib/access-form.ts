import type { Access, AccessMode, UpdateAccessRequest } from "@/api/types";
import { changedFields } from "./diff";

export interface AccessDraft {
  mode: AccessMode;
  emailAllowlist: string[];
  domainAllowlist: string[];
  password: string;
}

export const MIN_PASSWORD_LENGTH = 8;
export const MAX_PASSWORD_LENGTH = 128;

export function draftFromAccess(access: Access): AccessDraft {
  return { mode: access.mode, emailAllowlist: [...access.emailAllowlist], domainAllowlist: [...access.domainAllowlist], password: "" };
}

function listedFields(draft: AccessDraft): Pick<AccessDraft, "mode" | "emailAllowlist" | "domainAllowlist"> {
  return { mode: draft.mode, emailAllowlist: draft.emailAllowlist, domainAllowlist: draft.domainAllowlist };
}

export function buildAccessPatch(access: Access, draft: AccessDraft): UpdateAccessRequest {
  const patch: UpdateAccessRequest = changedFields(listedFields(draftFromAccess(access)), listedFields(draft));
  if (draft.password !== "") {
    patch.password = draft.password;
  }
  return patch;
}

export function accessProblem(access: Access, draft: AccessDraft): string | null {
  if (draft.password !== "" && (draft.password.length < MIN_PASSWORD_LENGTH || draft.password.length > MAX_PASSWORD_LENGTH)) {
    return `A password needs ${MIN_PASSWORD_LENGTH} to ${MAX_PASSWORD_LENGTH} characters.`;
  }
  if (draft.mode === "password" && !access.hasPassword && draft.password === "") {
    return "Set a password before switching this site to password mode.";
  }
  return null;
}
