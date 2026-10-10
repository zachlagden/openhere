import { useMemo, type JSX } from "react";
import { describeError, formatWait } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { useCountdown, type Wait } from "@/lib/use-countdown";

export function useRetryWait(error: unknown): Wait | null {
  return useMemo(() => {
    if (!error) {
      return null;
    }
    const { retryAfter } = describeError(error);
    return retryAfter === null ? null : { seconds: retryAfter };
  }, [error]);
}

export function useRateLimit(error: unknown): number {
  return useCountdown(useRetryWait(error));
}

interface FormErrorProps {
  error: unknown;
  remaining?: number;
}

export function FormError({ error, remaining = 0 }: FormErrorProps): JSX.Element | null {
  if (!error) {
    return null;
  }
  const description = describeError(error);
  if (description.kind === "rate_limited" && description.retryAfter !== null) {
    const message =
      remaining > 0 ? `Too many attempts. Try again in ${formatWait(remaining)}.` : "You can try again now.";
    return <Alert tone={remaining > 0 ? "error" : "info"}>{message}</Alert>;
  }
  return <Alert tone="error">{description.message}</Alert>;
}
