import { Copy } from "lucide-react";
import type { JSX } from "react";
import { Button } from "@/components/ui/button";
import { useToast } from "@/components/ui/toast";
import { copyText } from "@/lib/clipboard";

interface CopyButtonProps {
  value: string;
  label: string;
  done?: string;
}

export function CopyButton({ value, label, done = "Copied." }: CopyButtonProps): JSX.Element {
  const toast = useToast();
  async function copy(): Promise<void> {
    if (await copyText(value)) {
      toast.success(done);
    } else {
      toast.error("Could not copy. Select the text and copy it by hand.");
    }
  }
  return (
    <Button variant="ghost" size="icon" className="size-8 shrink-0" aria-label={label} title={label} onClick={() => void copy()}>
      <Copy />
    </Button>
  );
}

interface CopyFieldProps {
  label: string;
  value: string;
  copyLabel?: string;
  testId?: string;
}

export function CopyField({ label, value, copyLabel, testId }: CopyFieldProps): JSX.Element {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-medium uppercase tracking-wider text-muted">{label}</span>
      <div className="flex items-center gap-1 rounded-md border border-line bg-sunken pl-3">
        <code data-testid={testId} className="min-w-0 flex-1 break-all py-2 font-mono text-[0.8125rem]">
          {value}
        </code>
        <CopyButton value={value} label={copyLabel ?? `Copy ${label.toLowerCase()}`} done={`${label} copied.`} />
      </div>
    </div>
  );
}
