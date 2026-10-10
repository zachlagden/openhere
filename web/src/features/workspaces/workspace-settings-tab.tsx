import { useState, type FormEvent, type JSX } from "react";
import { useNavigate } from "react-router";
import { isApiError } from "@/api/client";
import { describeError } from "@/api/errors";
import type { Workspace } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/field";
import { SaveBar } from "@/components/ui/save-bar";
import { Section } from "@/components/ui/section";
import { useToast } from "@/components/ui/toast";
import { TypedConfirmDialog } from "@/components/ui/typed-confirm-dialog";
import { FormError } from "@/features/auth/form-error";
import { canRename } from "@/lib/workspace-access";
import { useDeleteWorkspace, useRenameWorkspace } from "@/queries/workspaces";
import { useWorkspaceContext } from "./workspace-context";

function describeDeleteError(error: unknown): string {
  if (isApiError(error) && error.code === "workspace_not_empty") {
    return "This workspace still contains sites. Delete them or ask their owners to, then try again.";
  }
  if (isApiError(error) && error.code === "personal_workspace") {
    return "A personal workspace cannot be deleted.";
  }
  return describeError(error).message;
}

function RenameForm({ workspace }: { workspace: Workspace }): JSX.Element {
  const rename = useRenameWorkspace(workspace.slug);
  const toast = useToast();
  const [name, setName] = useState(workspace.name);
  const trimmed = name.trim();
  const dirty = trimmed !== workspace.name && trimmed !== "";

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    rename.mutate(trimmed, { onSuccess: () => toast.success("Workspace renamed.") });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <TextField label="Name" value={name} onChange={(event) => setName(event.target.value)} maxLength={64} required autoComplete="off" hint="The address of the workspace, its slug, never changes." />
      <FormError error={rename.error} />
      <SaveBar dirty={dirty} pending={rename.isPending} onDiscard={() => setName(workspace.name)} />
    </form>
  );
}

function DeleteWorkspace({ workspace }: { workspace: Workspace }): JSX.Element {
  const [open, setOpen] = useState(false);
  const remove = useDeleteWorkspace(workspace.slug);
  const navigate = useNavigate();
  const toast = useToast();

  async function confirmDelete(): Promise<void> {
    try {
      await remove.mutateAsync();
    } catch {
      return;
    }
    setOpen(false);
    toast.success(`Deleted ${workspace.name}.`);
    void navigate("/", { replace: true });
  }

  if (workspace.kind === "personal") {
    return <p className="text-sm text-muted">A personal workspace belongs to your account and cannot be deleted.</p>;
  }

  return (
    <>
      <p className="text-sm text-muted">
        A workspace can only be deleted when it holds no sites. Delete or move its sites first. Deleting removes its folders and every member&apos;s access to it. Accounts stay.
      </p>
      <div>
        <Button variant="danger" onClick={() => setOpen(true)}>
          Delete this workspace
        </Button>
      </div>
      <TypedConfirmDialog
        open={open}
        onOpenChange={(next) => {
          if (!next) {
            remove.reset();
          }
          setOpen(next);
        }}
        title={`Delete ${workspace.name}?`}
        phrase={workspace.slug}
        confirmLabel="Delete workspace"
        pending={remove.isPending}
        error={remove.error ? describeDeleteError(remove.error) : null}
        onConfirm={() => void confirmDelete()}
      >
        <p>This cannot be undone. It works only while the workspace has no sites.</p>
      </TypedConfirmDialog>
    </>
  );
}

export function WorkspaceSettingsTab(): JSX.Element {
  const { workspace } = useWorkspaceContext();
  if (!canRename(workspace)) {
    return <Alert tone="info">Only workspace admins can change these settings.</Alert>;
  }
  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <Section title="Name">
        <RenameForm workspace={workspace} />
      </Section>
      <Section title="Delete workspace">
        <DeleteWorkspace workspace={workspace} />
      </Section>
    </div>
  );
}
