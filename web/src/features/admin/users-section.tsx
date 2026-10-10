import { useState, type JSX } from "react";
import type { AdminUser, CreatedInvite, ResetLink } from "@/api/types";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import { SecretDialog } from "@/components/ui/secret-dialog";
import { useToast } from "@/components/ui/toast";
import { describeUserError } from "@/lib/admin-users";
import { describeEmailOutcome } from "@/lib/invites";
import { formatDateTime } from "@/lib/format";
import { useAdminUsers, useCreateResetLink, useSetUserDisabled } from "@/queries/admin";
import { useMe } from "@/queries/session";
import { InviteUserForm } from "./invite-user-form";
import { UsersTable } from "./users-table";

type Pending = { kind: "disable" | "enable"; user: AdminUser } | null;

export function UsersSection(): JSX.Element {
  const users = useAdminUsers();
  const me = useMe();
  const setDisabled = useSetUserDisabled();
  const resetLink = useCreateResetLink();
  const toast = useToast();
  const [pending, setPending] = useState<Pending>(null);
  const [reset, setReset] = useState<{ user: AdminUser; link: ResetLink } | null>(null);
  const [resetError, setResetError] = useState<string | null>(null);
  const [invite, setInvite] = useState<{ created: CreatedInvite; askedForEmail: boolean } | null>(null);

  function confirm(): void {
    if (!pending) {
      return;
    }
    const target = pending;
    setDisabled.mutate(
      { id: target.user.id, disabled: target.kind === "disable" },
      {
        onSuccess: () => {
          setPending(null);
          toast.success(target.kind === "disable" ? `Disabled ${target.user.email}.` : `Enabled ${target.user.email}.`);
        },
      },
    );
  }

  function makeResetLink(user: AdminUser): void {
    setResetError(null);
    resetLink.mutate(user.id, {
      onSuccess: (link) => {
        setReset({ user, link });
        resetLink.reset();
      },
      onError: (error) => {
        setResetError(describeUserError(error));
        toast.error(describeUserError(error));
      },
    });
  }

  return (
    <div className="flex max-w-5xl flex-col gap-6">
      <Section title="Accounts" description="Disabling an account signs the person out and stops their API keys at once. Enabling restores the keys, not the sessions. The API cannot delete accounts.">
        {users.isPending ? <BlockSkeleton label="Loading accounts" rows={3} /> : null}
        {users.isError ? <LoadError error={users.error} onRetry={() => void users.refetch()} /> : null}
        {users.data ? (
          <UsersTable
            users={users.data}
            currentUserId={me.data?.id}
            onDisable={(user) => setPending({ kind: "disable", user })}
            onEnable={(user) => setPending({ kind: "enable", user })}
            onReset={makeResetLink}
          />
        ) : null}
        <p role="status" className="text-sm text-muted empty:hidden">
          {resetLink.isPending ? "Creating a reset link…" : (resetError ?? "")}
        </p>
      </Section>
      <Section title="Invite a user" description="Creates a link the person uses to set up their account.">
        <InviteUserForm onCreated={(created, askedForEmail) => setInvite({ created, askedForEmail })} />
      </Section>
      <SecretDialog
        title="Password reset link"
        fieldLabel="Reset link"
        secret={reset?.link.link ?? null}
        warning="This link is shown once. Pass it to the person yourself. It works once and replaces any earlier link."
        onClose={() => setReset(null)}
      >
        {reset ? (
          <p className="text-sm text-muted">
            For {reset.user.email}. It expires {formatDateTime(reset.link.expiresAt)}.
          </p>
        ) : null}
      </SecretDialog>
      <SecretDialog
        title="Invite link"
        fieldLabel="Invite link"
        secret={invite?.created.link ?? null}
        warning="This link is shown once. Copy it now and send it to the person. Anyone with the link can create the account for that address."
        onClose={() => setInvite(null)}
      >
        {invite ? <p className="text-sm text-muted">{describeEmailOutcome(invite.created, invite.askedForEmail)}</p> : null}
      </SecretDialog>
      <ConfirmDialog
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPending(null);
            setDisabled.reset();
          }
        }}
        title={pending ? `${pending.kind === "disable" ? "Disable" : "Enable"} ${pending.user.email}?` : "Change account"}
        confirmLabel={pending?.kind === "disable" ? "Disable account" : "Enable account"}
        tone={pending?.kind === "disable" ? "danger" : "primary"}
        pending={setDisabled.isPending}
        error={setDisabled.error ? describeUserError(setDisabled.error) : null}
        onConfirm={confirm}
      >
        {pending?.kind === "disable" ? (
          <p>They are signed out at once and their API keys stop working. Their sites stay as they are. You can enable the account again.</p>
        ) : (
          <p>They can sign in again and their API keys work again. Their old sessions do not come back.</p>
        )}
      </ConfirmDialog>
    </div>
  );
}
