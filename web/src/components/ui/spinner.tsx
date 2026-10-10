import { LoaderCircle } from "lucide-react";
import type { JSX } from "react";
import { cn } from "@/lib/cn";

export function Spinner({ label, className }: { label?: string; className?: string }): JSX.Element {
  return (
    <span role="status" className="inline-flex items-center gap-2 text-sm text-muted">
      <LoaderCircle aria-hidden="true" className={cn("size-4 animate-spin", className)} />
      {label ? <span>{label}</span> : <span className="sr-only">Loading</span>}
    </span>
  );
}
