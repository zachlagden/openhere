import { useCallback, useEffect, useRef, useState } from "react";

export type CopyStatus = "idle" | "copied" | "failed";

export const COPY_STATUS_MS = 2000;

interface CopyStatusControl {
  status: CopyStatus;
  run: (copy: () => Promise<boolean>) => Promise<void>;
}

export function useCopyStatus(): CopyStatusControl {
  const [status, setStatus] = useState<CopyStatus>("idle");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mounted = useRef(true);

  const clearTimer = useCallback((): void => {
    if (timer.current !== null) {
      clearTimeout(timer.current);
      timer.current = null;
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      clearTimer();
    };
  }, [clearTimer]);

  const run = useCallback(
    async (copy: () => Promise<boolean>): Promise<void> => {
      const succeeded = await copy();
      if (!mounted.current) {
        return;
      }
      clearTimer();
      setStatus(succeeded ? "copied" : "failed");
      timer.current = setTimeout(() => {
        timer.current = null;
        setStatus("idle");
      }, COPY_STATUS_MS);
    },
    [clearTimer],
  );

  return { status, run };
}

export function copyStatusText(status: CopyStatus): string {
  if (status === "copied") {
    return "Copied";
  }
  if (status === "failed") {
    return "Copy failed";
  }
  return "";
}
