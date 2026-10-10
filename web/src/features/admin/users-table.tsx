import type { JSX } from "react";
import type { AdminUser } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type Column } from "@/components/ui/data-table";
import { canDisable, canResetPassword, sortUsers, userNote } from "@/lib/admin-users";
import { formatDate } from "@/lib/format";

interface UsersTableProps {
  users: AdminUser[];
  currentUserId: string | undefined;
  onDisable: (user: AdminUser) => void;
  onEnable: (user: AdminUser) => void;
  onReset: (user: AdminUser) => void;
}

export function UsersTable({ users, currentUserId, onDisable, onEnable, onReset }: UsersTableProps): JSX.Element {
  const columns: Column<AdminUser>[] = [
    {
      key: "person",
      header: "Account",
      cell: (user) => (
        <span className="flex flex-col">
          <span className="break-all font-medium">{user.email}</span>
          {user.displayName ? <span className="text-xs text-muted">{user.displayName}</span> : null}
          {userNote(user, currentUserId) ? <span className="text-xs text-muted">{userNote(user, currentUserId)}</span> : null}
        </span>
      ),
    },
    { key: "role", header: "Role", cell: (user) => (user.isRoot ? <Badge tone="accent">Root</Badge> : "Member") },
    { key: "status", header: "Status", cell: (user) => <Badge tone={user.disabled ? "danger" : "ok"}>{user.disabled ? "Disabled" : "Active"}</Badge> },
    { key: "created", header: "Created", cell: (user) => formatDate(user.createdAt) },
    {
      key: "actions",
      header: "Actions",
      hideLabelOnPhone: true,
      className: "md:text-right",
      cell: (user) => (
        <span className="inline-flex flex-wrap justify-end gap-1.5">
          {canResetPassword(user) ? (
            <Button variant="secondary" size="sm" onClick={() => onReset(user)} aria-label={`Create a reset link for ${user.email}`}>
              Reset link
            </Button>
          ) : null}
          {user.disabled ? (
            <Button variant="secondary" size="sm" onClick={() => onEnable(user)} aria-label={`Enable ${user.email}`}>
              Enable
            </Button>
          ) : canDisable(user, currentUserId) ? (
            <Button variant="ghost" size="sm" onClick={() => onDisable(user)} aria-label={`Disable ${user.email}`}>
              Disable
            </Button>
          ) : null}
        </span>
      ),
    },
  ];
  return <DataTable caption="Accounts on this server" columns={columns} rows={sortUsers(users)} rowKey={(user) => user.id} />;
}
