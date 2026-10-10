import { useState, type FormEvent, type JSX } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogFooter } from "@/components/ui/dialog";
import { TextField } from "@/components/ui/field";
import { Alert } from "@/components/ui/alert";
import { MAX_LABEL_LENGTH, describeKeyError, validateLabel } from "@/lib/api-keys";
import { useCreateApiKey } from "@/queries/api-keys";

interface CreateKeyDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (key: string) => void;
}

export function CreateKeyDialog({ open, onOpenChange, onCreated }: CreateKeyDialogProps): JSX.Element {
  const create = useCreateApiKey();
  const [label, setLabel] = useState("");
  const [problem, setProblem] = useState<string | null>(null);

  function close(next: boolean): void {
    if (!next) {
      create.reset();
      setLabel("");
      setProblem(null);
    }
    onOpenChange(next);
  }

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const invalid = validateLabel(label);
    setProblem(invalid);
    if (invalid !== null) {
      return;
    }
    create.mutate(label.trim(), {
      onSuccess: (created) => {
        onCreated(created.key);
        close(false);
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={close} title="Create an API key" description="A key has the full access of your account. Give it a label that says where it will be used.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        <TextField label="Label" value={label} onChange={(event) => setLabel(event.target.value)} maxLength={MAX_LABEL_LENGTH} placeholder="ci runner" autoComplete="off" autoFocus error={problem} />
        {create.isError ? <Alert tone="error">{describeKeyError(create.error)}</Alert> : null}
        <DialogFooter>
          <Button variant="secondary" onClick={() => close(false)}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={create.isPending}>
            {create.isPending ? "Creating…" : "Create key"}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}
