import { isApiError } from "@/api/client";
import { describeError } from "@/api/errors";
import type { CreatedInvite, Invite } from "@/api/types";

export type InviteTone = "ok" | "warn" | "neutral" | "danger";

export function inviteStatus(invite: Invite): { label: string; tone: InviteTone } {
  switch (invite.status) {
    case "pending":
      return { label: "Waiting", tone: "warn" };
    case "accepted":
      return { label: "Accepted", tone: "ok" };
    case "revoked":
      return { label: "Revoked", tone: "neutral" };
    case "expired":
      return { label: "Expired", tone: "danger" };
  }
}

export function canRevoke(invite: Invite): boolean {
  return invite.status === "pending";
}

export function canResend(invite: Invite): boolean {
  return invite.status === "pending" || invite.status === "expired" || invite.status === "revoked";
}

export function invitesForWorkspace(invites: Invite[], workspaceId: string): Invite[] {
  return invites.filter((invite) => invite.workspace?.id === workspaceId);
}

const INVITE_MESSAGES: Record<string, string> = {
  email_registered: "That address already has an account. Add the person from the Members tab instead.",
  personal_workspace: "A personal workspace cannot take members.",
  invite_accepted: "That invite was already accepted.",
};

export function describeInviteError(error: unknown): string {
  if (isApiError(error)) {
    if (error.code in INVITE_MESSAGES) {
      return INVITE_MESSAGES[error.code] ?? error.message;
    }
    if (error.status === 403) {
      return "You are not allowed to send invites for this workspace. Ask the root user, or check the invite setting.";
    }
  }
  return describeError(error).message;
}

export function describeEmailOutcome(created: CreatedInvite, askedForEmail: boolean): string | null {
  if (created.emailSent) {
    return `The link was also emailed to ${created.invite.email}.`;
  }
  if (askedForEmail && created.emailError) {
    return `The invite exists, but the email could not be sent: ${created.emailError}. Send the link yourself.`;
  }
  if (!created.emailAvailable) {
    return "This server cannot send email, so send the link yourself.";
  }
  return null;
}
