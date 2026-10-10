import { MailPlus } from "lucide-react";
import { useState, type JSX } from "react";
import type { CreatedInvite, Invite } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Dialog, DialogFooter } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import { SecretDialog } from "@/components/ui/secret-dialog";
import { SwitchField } from "@/components/ui/switch";
import { useToast } from "@/components/ui/toast";
import { describeEmailOutcome, describeInviteError, invitesForWorkspace } from "@/lib/invites";
import { canInvite } from "@/lib/workspace-access";
import { useInstance } from "@/queries/instance";
import { useInvites, useResendInvite, useRevokeInvite } from "@/queries/invites";
import { CreateInviteForm } from "./create-invite-form";
import { InvitesTable } from "./invites-table";
import { useWorkspaceContext } from "./workspace-context";

interface Issued {
  created: CreatedInvite;
  askedForEmail: boolean;
  resent: boolean;
}

function ResendDialog({ invite, emailAvailable, onClose, onIssued }: { invite: Invite | null; emailAvailable: boolean; onClose: () => void; onIssued: (issued: Issued) => void }): JSX.Element {
  const resend = useResendInvite();
  const [sendEmail, setSendEmail] = useState(false);
  return (
    <Dialog
      open={invite !== null}
      onOpenChange={(open) => {
        if (!open) {
          resend.reset();
          onClose();
        }
      }}
      title={invite ? `Resend the invite to ${invite.email}?` : "Resend invite"}
    >
      <div className="flex flex-col gap-4">
        <p className="text-sm text-muted">This replaces the old link with a new one that lasts 7 days. The old link stops working.</p>
        {emailAvailable ? <SwitchField label="Email the new link" checked={sendEmail} onChange={(event) => setSendEmail(event.target.checked)} /> : null}
        {resend.isError ? <Alert tone="error">{describeInviteError(resend.error)}</Alert> : null}
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            disabled={resend.isPending || invite === null}
            onClick={() => {
              if (invite) {
                resend.mutate(
                  { id: invite.id, sendEmail: emailAvailable && sendEmail },
                  {
                    onSuccess: (created) => {
                      onIssued({ created, askedForEmail: emailAvailable && sendEmail, resent: true });
                      resend.reset();
                    },
                  },
                );
              }
            }}
          >
            {resend.isPending ? "Sending…" : "Create new link"}
          </Button>
        </DialogFooter>
      </div>
    </Dialog>
  );
}

export function InvitesTab(): JSX.Element {
  const { workspace, isRoot } = useWorkspaceContext();
  const instance = useInstance();
  const allowed = canInvite(workspace, isRoot, instance.data?.whoCanInvite);
  const invites = useInvites(allowed);
  const revoke = useRevokeInvite();
  const toast = useToast();
  const [issued, setIssued] = useState<Issued | null>(null);
  const [revoking, setRevoking] = useState<Invite | null>(null);
  const [resending, setResending] = useState<Invite | null>(null);
  const emailAvailable = instance.data?.emailAvailable ?? false;

  if (!allowed) {
    return (
      <Alert tone="info">
        {workspace.kind === "personal"
          ? "Personal workspaces have no other members, so they take no invites."
          : instance.data?.whoCanInvite === "root"
            ? "This server lets only the root user send invites. Ask them to invite the person, or add someone who already has an account from the Members tab."
            : "Only workspace admins can send invites."}
      </Alert>
    );
  }

  const list = invitesForWorkspace(invites.data ?? [], workspace.id);

  function confirmRevoke(): void {
    if (!revoking) {
      return;
    }
    const target = revoking;
    revoke.mutate(target.id, {
      onSuccess: () => {
        setRevoking(null);
        toast.success(`Revoked the invite to ${target.email}.`);
      },
    });
  }

  return (
    <div className="flex max-w-5xl flex-col gap-6">
      <Section title="Invite someone new" description="They create an account from the link and join this workspace with the role you choose.">
        <CreateInviteForm workspace={workspace} emailAvailable={emailAvailable} onCreated={(created, askedForEmail) => setIssued({ created, askedForEmail, resent: false })} />
      </Section>
      <Section title="Invites for this workspace" description={isRoot ? "You see every invite for this workspace." : "You see the invites you sent."}>
        {invites.isPending ? <BlockSkeleton label="Loading invites" rows={2} /> : null}
        {invites.isError ? <LoadError error={invites.error} onRetry={() => void invites.refetch()} /> : null}
        {invites.data && list.length === 0 ? (
          <EmptyState icon={<MailPlus />} title="No invites yet">
            <p>Invites you create for this workspace appear here. The link is shown once, when you create it, so copy it then.</p>
          </EmptyState>
        ) : null}
        {list.length > 0 ? <InvitesTable invites={list} onResend={setResending} onRevoke={setRevoking} /> : null}
      </Section>
      <SecretDialog
        title={issued?.resent ? "New invite link" : "Invite link"}
        fieldLabel="Invite link"
        secret={issued?.created.link ?? null}
        warning="This link is shown once. Copy it now and send it to the person. Anyone with the link can create the account for that address."
        onClose={() => setIssued(null)}
      >
        {issued ? <p className="text-sm text-muted">{describeEmailOutcome(issued.created, issued.askedForEmail)}</p> : null}
      </SecretDialog>
      <ResendDialog
        invite={resending}
        emailAvailable={emailAvailable}
        onClose={() => setResending(null)}
        onIssued={(next) => {
          setResending(null);
          setIssued(next);
        }}
      />
      <ConfirmDialog
        open={revoking !== null}
        onOpenChange={(open) => {
          if (!open) {
            setRevoking(null);
            revoke.reset();
          }
        }}
        title={revoking ? `Revoke the invite to ${revoking.email}?` : "Revoke invite"}
        confirmLabel="Revoke invite"
        pending={revoke.isPending}
        error={revoke.error ? describeInviteError(revoke.error) : null}
        onConfirm={confirmRevoke}
      >
        <p>The link stops working at once. You can send a new invite later.</p>
      </ConfirmDialog>
    </div>
  );
}
