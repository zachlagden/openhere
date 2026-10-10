import { useEffect, useState } from "react";

export interface Wait {
  seconds: number;
}

export function useCountdown(wait: Wait | null): number {
  const [remaining, setRemaining] = useState(wait?.seconds ?? 0);
  const [source, setSource] = useState(wait);
  if (source !== wait) {
    setSource(wait);
    setRemaining(wait?.seconds ?? 0);
  }
  useEffect(() => {
    if (remaining <= 0) {
      return undefined;
    }
    const timer = setTimeout(() => setRemaining((value) => Math.max(0, value - 1)), 1000);
    return () => clearTimeout(timer);
  }, [remaining]);
  return remaining;
}
