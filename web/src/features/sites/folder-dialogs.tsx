import type { FormEvent, JSX } from "react";
import type { Folder } from "@/api/types";
import { describeError } from "@/api/errors";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Dialog, DialogFooter } from "@/components/ui/dialog";
import { TextField } from "@/components/ui/field";
import { useToast } from "@/components/ui/toast";
import { FormError } from "@/features/auth/form-error";
import { useCreateFolder, useDeleteFolder, useRenameFolder } from "@/queries/folders";

interface DialogBase {
  workspace: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function readName(event: FormEvent<HTMLFormElement>): string {
  event.preventDefault();
  return String(new FormData(event.currentTarget).get("name") ?? "").trim();
}

export function CreateFolderDialog({ workspace, open, onOpenChange }: DialogBase): JSX.Element {
  const create = useCreateFolder(workspace);
  const toast = useToast();

  function submit(event: FormEvent<HTMLFormElement>): void {
    create.mutate(readName(event), {
      onSuccess: (folder) => {
        onOpenChange(false);
        toast.success(`Created folder ${folder.name}.`);
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="New folder" description="Folders group sites in this workspace. They cannot contain other folders.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        <TextField label="Name" name="name" required maxLength={64} autoComplete="off" autoFocus placeholder="Reports" />
        <FormError error={create.error} />
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={create.isPending}>
            {create.isPending ? "Creating…" : "Create folder"}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}

export function RenameFolderDialog({ workspace, folder, open, onOpenChange }: DialogBase & { folder: Folder }): JSX.Element {
  const rename = useRenameFolder(workspace);
  const toast = useToast();

  function submit(event: FormEvent<HTMLFormElement>): void {
    const name = readName(event);
    if (name === folder.name) {
      onOpenChange(false);
      return;
    }
    rename.mutate(
      { id: folder.id, name },
      {
        onSuccess: () => {
          onOpenChange(false);
          toast.success("Folder renamed.");
        },
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Rename folder">
      <form onSubmit={submit} className="flex flex-col gap-4">
        <TextField label="Name" name="name" required maxLength={64} autoComplete="off" autoFocus defaultValue={folder.name} />
        <FormError error={rename.error} />
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={rename.isPending}>
            {rename.isPending ? "Saving…" : "Save name"}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}

export function DeleteFolderDialog({ workspace, folder, open, onOpenChange, onDeleted }: DialogBase & { folder: Folder; onDeleted: () => void }): JSX.Element {
  const remove = useDeleteFolder(workspace);
  const toast = useToast();

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Delete ${folder.name}?`}
      confirmLabel="Delete folder"
      pending={remove.isPending}
      error={remove.error ? describeError(remove.error).message : null}
      onConfirm={() =>
        remove.mutate(folder.id, {
          onSuccess: () => {
            onOpenChange(false);
            toast.success("Folder deleted. Its sites are now unfiled.");
            onDeleted();
          },
        })
      }
    >
      <p>The sites in this folder stay in the workspace and become unfiled. This cannot be undone.</p>
    </ConfirmDialog>
  );
}
