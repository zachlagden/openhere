import { useState, type JSX } from "react";
import { Link } from "react-router";
import { workspacePath } from "@/api/paths";
import type { Member, Role } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import { useToast } from "@/components/ui/toast";
import { adminCount, describeMemberError, scopeLabel } from "@/lib/members";
import { canManageMembers } from "@/lib/workspace-access";
import { useMembers, useRemoveMember, useUpdateMember } from "@/queries/members";
import { AddMemberFormSection } from "./add-member-form";
import { MembersTable } from "./members-table";
import { useWorkspaceContext } from "./workspace-context";

export function MembersTab(): JSX.Element {
  const { workspace, userEmail } = useWorkspaceContext();
  const allowed = canManageMembers(workspace);
  const members = useMembers(workspace.slug, allowed);
  const update = useUpdateMember(workspace.slug);
  const remove = useRemoveMember(workspace.slug);
  const toast = useToast();
  const [removing, setRemoving] = useState<Member | null>(null);

  if (!allowed) {
    return (
      <Alert tone="info">
        {workspace.kind === "personal" ? "Personal workspaces have no other members." : "Only workspace admins can see and change members."}
      </Alert>
    );
  }

  function changeRole(member: Member, role: Role): void {
    update.mutate(
      { id: member.id, role },
      {
        onSuccess: () => toast.success(`${member.email} is now ${role}.`),
        onError: (error) => toast.error(describeMemberError(error)),
      },
    );
  }

  function confirmRemove(): void {
    if (removing === null) {
      return;
    }
    const target = removing;
    remove.mutate(target.id, {
      onSuccess: () => {
        setRemoving(null);
        toast.success(`Removed ${target.email}.`);
      },
    });
  }

  const admins = members.data ? adminCount(members.data) : 0;

  return (
    <div className="flex max-w-5xl flex-col gap-6">
      <Section
        title="People with access"
        description="A person can hold several grants: one for the whole workspace, and more for single folders or sites. Admin applies to the whole workspace only."
      >
        <p className="text-sm text-muted">
          A workspace always keeps at least one admin{admins === 1 ? ", and right now there is one" : ""}. The server refuses a change that would remove the last one.
        </p>
        {members.isPending ? <BlockSkeleton label="Loading members" rows={3} /> : null}
        {members.isError ? <LoadError error={members.error} onRetry={() => void members.refetch()} /> : null}
        {members.data ? (
          <MembersTable members={members.data} currentEmail={userEmail} busyId={update.isPending ? (update.variables?.id ?? null) : null} onChangeRole={changeRole} onRemove={setRemoving} />
        ) : null}
      </Section>
      <Section
        title="Add a member"
        description={
          <>
            Add someone who already has an account. For someone new, <Link className="underline underline-offset-4" to={`${workspacePath(workspace.slug)}/invites`}>send an invite</Link> instead.
          </>
        }
      >
        <AddMemberFormSection workspace={workspace.slug} />
      </Section>
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => {
          if (!open) {
            setRemoving(null);
            remove.reset();
          }
        }}
        title={removing ? `Remove ${removing.email}?` : "Remove member?"}
        confirmLabel="Remove access"
        pending={remove.isPending}
        error={remove.error ? describeMemberError(remove.error) : null}
        onConfirm={confirmRemove}
      >
        <p>{removing ? `This removes their access to: ${scopeLabel(removing).toLowerCase()}. Their account and any other grants stay.` : ""}</p>
      </ConfirmDialog>
    </div>
  );
}
