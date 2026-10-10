import type { JSX, ReactNode } from "react";
import { Alert } from "./alert";
import { Button } from "./button";
import { Dialog, DialogFooter } from "./dialog";

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  confirmLabel: string;
  pending: boolean;
  error: string | null;
  tone?: "danger" | "primary";
  onConfirm: () => void;
  children: ReactNode;
}

export function ConfirmDialog({ open, onOpenChange, title, confirmLabel, pending, error, tone = "danger", onConfirm, children }: ConfirmDialogProps): JSX.Element {
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={title}>
      <div className="text-sm text-muted">{children}</div>
      {error ? <Alert tone="error">{error}</Alert> : null}
      <DialogFooter>
        <Button variant="secondary" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        <Button variant={tone} disabled={pending} onClick={onConfirm}>
          {pending ? "Working…" : confirmLabel}
        </Button>
      </DialogFooter>
    </Dialog>
  );
}
