import type { JSX } from "react";
import type { Invite } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type Column } from "@/components/ui/data-table";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { canResend, canRevoke, inviteStatus } from "@/lib/invites";
import { roleLabel } from "@/lib/roles";

interface InvitesTableProps {
  invites: Invite[];
  onResend: (invite: Invite) => void;
  onRevoke: (invite: Invite) => void;
}

export function InvitesTable({ invites, onResend, onRevoke }: InvitesTableProps): JSX.Element {
  const columns: Column<Invite>[] = [
    { key: "email", header: "Email", cell: (invite) => <span className="break-all">{invite.email}</span> },
    { key: "role", header: "Role", cell: (invite) => (invite.role ? roleLabel(invite.role) : "None") },
    {
      key: "status",
      header: "Status",
      cell: (invite) => {
        const status = inviteStatus(invite);
        return <Badge tone={status.tone}>{status.label}</Badge>;
      },
    },
    {
      key: "expires",
      header: "Expires",
      cell: (invite) => (
        <time dateTime={invite.expiresAt} title={formatDateTime(invite.expiresAt)}>
          {formatRelativeTime(invite.expiresAt)}
        </time>
      ),
    },
    { key: "by", header: "Invited by", cell: (invite) => <span className="break-all">{invite.invitedBy}</span> },
    {
      key: "actions",
      header: "Actions",
      hideLabelOnPhone: true,
      className: "md:text-right",
      cell: (invite) => (
        <span className="inline-flex gap-1.5">
          {canResend(invite) ? (
            <Button variant="secondary" size="sm" onClick={() => onResend(invite)} aria-label={`Resend invite to ${invite.email}`}>
              Resend
            </Button>
          ) : null}
          {canRevoke(invite) ? (
            <Button variant="ghost" size="sm" onClick={() => onRevoke(invite)} aria-label={`Revoke invite to ${invite.email}`}>
              Revoke
            </Button>
          ) : null}
        </span>
      ),
    },
  ];
  return <DataTable caption="Invites for this workspace" columns={columns} rows={invites} rowKey={(invite) => invite.id} />;
}
