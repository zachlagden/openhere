import { useState, type FormEvent, type JSX, type ReactNode } from "react";
import { Alert } from "./alert";
import { Button } from "./button";
import { Dialog, DialogFooter } from "./dialog";
import { TextField } from "./field";

interface TypedConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  phrase: string;
  confirmLabel: string;
  pending: boolean;
  error: string | null;
  onConfirm: () => void;
  children: ReactNode;
}

export function TypedConfirmDialog({ open, onOpenChange, title, phrase, confirmLabel, pending, error, onConfirm, children }: TypedConfirmDialogProps): JSX.Element {
  const [typed, setTyped] = useState("");
  const matches = typed.trim() === phrase;

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    if (matches) {
      onConfirm();
    }
  }

  function change(next: boolean): void {
    if (!next) {
      setTyped("");
    }
    onOpenChange(next);
  }

  return (
    <Dialog open={open} onOpenChange={change} title={title}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        <div className="text-sm text-muted">{children}</div>
        <TextField label={`Type ${phrase} to confirm`} value={typed} onChange={(event) => setTyped(event.target.value)} autoComplete="off" autoFocus spellCheck={false} />
        {error ? <Alert tone="error">{error}</Alert> : null}
        <DialogFooter>
          <Button variant="secondary" onClick={() => change(false)}>
            Cancel
          </Button>
          <Button type="submit" variant="danger" disabled={!matches || pending}>
            {pending ? "Working…" : confirmLabel}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}
