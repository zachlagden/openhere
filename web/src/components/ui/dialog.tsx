import { X } from "lucide-react";
import { useEffect, useId, useRef, type JSX, type ReactNode } from "react";
import { cn } from "@/lib/cn";
import { Button } from "./button";

interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
}

export function Dialog({ open, onOpenChange, title, description, children, className }: DialogProps): JSX.Element {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    const element = ref.current;
    if (!element) {
      return;
    }
    if (open && !element.open) {
      element.showModal();
    }
    if (!open && element.open) {
      element.close();
    }
  }, [open]);

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      aria-describedby={description ? descriptionId : undefined}
      onClose={() => onOpenChange(false)}
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onOpenChange(false);
        }
      }}
      className={cn(
        "m-auto w-[calc(100vw-2rem)] overscroll-contain max-w-md rounded-xl border border-line bg-surface p-0 text-foreground shadow-dialog backdrop:bg-black/40",
        className,
      )}
    >
      {open ? (
        <div className="flex flex-col gap-4 p-5 sm:p-6">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <h2 id={titleId} className="font-display text-xl font-medium leading-snug">
                {title}
              </h2>
              {description ? (
                <p id={descriptionId} className="mt-1 text-sm text-muted">
                  {description}
                </p>
              ) : null}
            </div>
            <Button variant="ghost" size="icon" className="-mr-2 -mt-2" aria-label="Close dialog" onClick={() => onOpenChange(false)}>
              <X />
            </Button>
          </div>
          {children}
        </div>
      ) : null}
    </dialog>
  );
}

export function DialogFooter({ children }: { children: ReactNode }): JSX.Element {
  return <div className="mt-2 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">{children}</div>;
}
