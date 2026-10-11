import { useRef, type JSX } from "react";
import { CopyButton } from "@/components/ui/copy-button";

interface CopyFieldProps {
  label: string;
  value: string;
  copyLabel?: string;
  testId?: string;
}

export function CopyField({ label, value, copyLabel, testId }: CopyFieldProps): JSX.Element {
  const valueRef = useRef<HTMLElement>(null);

  function selectValue(): void {
    const element = valueRef.current;
    const selection = window.getSelection();
    if (element === null || selection === null) {
      return;
    }
    const range = document.createRange();
    range.selectNodeContents(element);
    selection.removeAllRanges();
    selection.addRange(range);
  }

  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-medium uppercase tracking-wider text-muted">{label}</span>
      <div className="flex items-center gap-1 rounded-md border border-line bg-sunken pl-3">
        <code ref={valueRef} data-testid={testId} className="min-w-0 flex-1 select-all break-all py-2 font-mono text-[0.8125rem]">
          {value}
        </code>
        <CopyButton value={value} label={copyLabel ?? `Copy ${label.toLowerCase()}`} onFailure={selectValue} />
      </div>
    </div>
  );
}
