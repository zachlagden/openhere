import { AlertTriangle, CircleAlert, Info } from "lucide-react";
import type { JSX, ReactNode } from "react";
import { cn } from "@/lib/cn";

type Tone = "info" | "warning" | "error";

const toneClasses: Record<Tone, string> = {
  info: "border-line bg-sunken text-foreground",
  warning: "border-transparent bg-warn-soft text-warn",
  error: "border-transparent bg-danger-soft text-danger",
};

const toneIcons = { info: Info, warning: AlertTriangle, error: CircleAlert } as const;

interface AlertProps {
  tone?: Tone;
  children: ReactNode;
  className?: string;
}

export function Alert({ tone = "info", children, className }: AlertProps): JSX.Element {
  const Icon = toneIcons[tone];
  return (
    <div
      role={tone === "error" ? "alert" : "status"}
      className={cn("flex items-start gap-2.5 rounded-md border px-3 py-2.5 text-sm", toneClasses[tone], className)}
    >
      <Icon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}
