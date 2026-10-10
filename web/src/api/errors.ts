import { isApiError, isNetworkError } from "./client";

export type ErrorKind = "network" | "rate_limited" | "unauthenticated" | "forbidden" | "not_found" | "other";

export interface ErrorDescription {
  kind: ErrorKind;
  message: string;
  retryAfter: number | null;
}

export function formatWait(seconds: number): string {
  const whole = Math.max(1, Math.ceil(seconds));
  if (whole < 90) {
    return whole === 1 ? "1 second" : `${whole} seconds`;
  }
  const minutes = Math.ceil(whole / 60);
  if (minutes < 90) {
    return `${minutes} minutes`;
  }
  const hours = Math.ceil(minutes / 60);
  return hours === 1 ? "1 hour" : `${hours} hours`;
}

const CODE_MESSAGES: Record<string, string> = {
  csrf_rejected: "The server blocked this request. Reload the page and try again.",
  internal_error: "Something went wrong on the server. Try again in a moment.",
  unauthenticated: "Your session has ended. Sign in again.",
  invalid_credentials: "Incorrect email or password.",
  body_too_large: "That request is too large.",
  unsupported_media_type: "The server could not read that request.",
};

export function describeError(error: unknown): ErrorDescription {
  if (isNetworkError(error)) {
    return { kind: "network", message: "Cannot reach the server. Check your connection and try again.", retryAfter: null };
  }
  if (!isApiError(error)) {
    return { kind: "other", message: "Something went wrong. Try again.", retryAfter: null };
  }
  if (error.status === 429 || error.code === "rate_limited") {
    const wait = error.retryAfter;
    const message = wait === null ? "Too many attempts. Try again later." : `Too many attempts. Try again in ${formatWait(wait)}.`;
    return { kind: "rate_limited", message, retryAfter: wait };
  }
  const message = CODE_MESSAGES[error.code] ?? error.message;
  return { kind: kindFor(error.status), message, retryAfter: null };
}

function kindFor(status: number): ErrorKind {
  switch (status) {
    case 401:
      return "unauthenticated";
    case 403:
      return "forbidden";
    case 404:
      return "not_found";
    default:
      return "other";
  }
}
