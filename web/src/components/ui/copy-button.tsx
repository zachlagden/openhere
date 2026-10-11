import { Check, Copy, X } from "lucide-react";
import type { JSX } from "react";
import { Button, type ButtonProps } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import { copyText } from "@/lib/clipboard";
import { copyStatusText, useCopyStatus, type CopyStatus } from "@/lib/use-copy-status";

function StatusIcon({ status }: { status: CopyStatus }): JSX.Element {
  if (status === "copied") {
    return <Check className="text-ok" />;
  }
  if (status === "failed") {
    return <X className="text-danger" />;
  }
  return <Copy />;
}

function LiveRegion({ status }: { status: CopyStatus }): JSX.Element {
  return (
    <span className="sr-only" aria-live="polite">
      {copyStatusText(status)}
    </span>
  );
}

interface CopyButtonProps {
  value: string;
  label: string;
  onFailure?: () => void;
  className?: string;
}

export function CopyButton({ value, label, onFailure, className }: CopyButtonProps): JSX.Element {
  const { status, run } = useCopyStatus();

  async function copy(): Promise<void> {
    await run(async () => {
      const succeeded = await copyText(value);
      if (!succeeded) {
        onFailure?.();
      }
      return succeeded;
    });
  }

  const name = status === "idle" ? label : copyStatusText(status);
  return (
    <>
      <Button variant="ghost" size="icon" className={cn("size-8 shrink-0", className)} aria-label={name} title={name} onClick={() => void copy()}>
        <StatusIcon status={status} />
      </Button>
      <LiveRegion status={status} />
    </>
  );
}

interface CopyLabelButtonProps {
  label: string;
  title?: string;
  status: CopyStatus;
  onCopy: () => void;
  variant?: ButtonProps["variant"];
  size?: ButtonProps["size"];
  labelClassName?: string;
}

export function CopyLabelButton({ label, title, status, onCopy, variant, size, labelClassName }: CopyLabelButtonProps): JSX.Element {
  const confirming = status !== "idle";
  const name = confirming ? copyStatusText(status) : label;
  return (
    <>
      <Button variant={variant} size={size} onClick={onCopy} aria-label={name} title={confirming ? name : title}>
        <StatusIcon status={status} />
        <span className={cn("grid", labelClassName)}>
          <span className={cn("col-start-1 row-start-1", confirming && "invisible")}>{label}</span>
          <span className={cn("col-start-1 row-start-1", !confirming && "invisible")}>{copyStatusText(status) || label}</span>
        </span>
      </Button>
      <LiveRegion status={status} />
    </>
  );
}
