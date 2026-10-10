import type { JSX } from "react";
import { Spinner } from "@/components/ui/spinner";

export function FullPageSpinner(): JSX.Element {
  return (
    <div className="flex min-h-dvh items-center justify-center">
      <Spinner label="Loading…" />
    </div>
  );
}
