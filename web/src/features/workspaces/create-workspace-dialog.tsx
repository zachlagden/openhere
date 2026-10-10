import type { FormEvent, JSX } from "react";
import { useNavigate } from "react-router";
import { Button } from "@/components/ui/button";
import { Dialog, DialogFooter } from "@/components/ui/dialog";
import { TextField } from "@/components/ui/field";
import { useToast } from "@/components/ui/toast";
import { FormError } from "@/features/auth/form-error";
import { useCreateWorkspace } from "@/queries/workspaces";

interface CreateWorkspaceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateWorkspaceDialog({ open, onOpenChange }: CreateWorkspaceDialogProps): JSX.Element {
  const create = useCreateWorkspace();
  const navigate = useNavigate();
  const toast = useToast();

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const name = String(new FormData(event.currentTarget).get("name") ?? "").trim();
    create.mutate(name, {
      onSuccess: (workspace) => {
        onOpenChange(false);
        toast.success(`Created ${workspace.name}.`);
        void navigate(`/workspaces/${encodeURIComponent(workspace.slug)}`);
      },
    });
  }

  function change(next: boolean): void {
    if (!next) {
      create.reset();
    }
    onOpenChange(next);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={change}
      title="Create a shared workspace"
      description="Members of a shared workspace share its sites and folders. You become its admin."
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <TextField label="Name" name="name" required maxLength={64} autoComplete="off" autoFocus placeholder="Acme Ltd" />
        <FormError error={create.error} />
        <DialogFooter>
          <Button variant="secondary" onClick={() => change(false)}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={create.isPending}>
            {create.isPending ? "Creating…" : "Create workspace"}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}
