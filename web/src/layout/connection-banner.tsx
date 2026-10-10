import { WifiOff } from "lucide-react";
import type { JSX } from "react";
import { useOnline } from "@/lib/use-online";

export function ConnectionBanner(): JSX.Element | null {
  const online = useOnline();
  if (online) {
    return null;
  }
  return (
    <div role="status" className="flex items-center justify-center gap-2 bg-warn-soft px-4 py-2 text-sm text-warn">
      <WifiOff aria-hidden="true" className="size-4" />
      You are offline. The dashboard will update when your connection returns.
    </div>
  );
}
