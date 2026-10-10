import type { JSX } from "react";
import type { Member, Role } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type Column } from "@/components/ui/data-table";
import { Select } from "@/components/ui/field";
import { roleOptions, scopeLabel, sortMembers } from "@/lib/members";
import { roleLabel } from "@/lib/roles";

function isRole(value: string): value is Role {
  return value === "admin" || value === "write" || value === "read";
}

interface MembersTableProps {
  members: Member[];
  currentEmail: string;
  busyId: string | null;
  onChangeRole: (member: Member, role: Role) => void;
  onRemove: (member: Member) => void;
}

export function MembersTable({ members, currentEmail, busyId, onChangeRole, onRemove }: MembersTableProps): JSX.Element {
  const columns: Column<Member>[] = [
    {
      key: "person",
      header: "Person",
      cell: (member) => (
        <span className="inline-flex flex-wrap items-center gap-1.5 break-all">
          {member.email}
          {member.email === currentEmail ? <Badge>You</Badge> : null}
        </span>
      ),
    },
    { key: "scope", header: "Access to", cell: (member) => scopeLabel(member) },
    {
      key: "role",
      header: "Role",
      cell: (member) => (
        <Select
          aria-label={`Role of ${member.email} for ${scopeLabel(member)}`}
          className="w-36"
          value={member.role}
          disabled={busyId === member.id}
          onChange={(event) => isRole(event.target.value) && onChangeRole(member, event.target.value)}
        >
          {roleOptions(member.scopeType).map((role) => (
            <option key={role} value={role}>
              {roleLabel(role)}
            </option>
          ))}
        </Select>
      ),
    },
    {
      key: "actions",
      header: "Actions",
      hideLabelOnPhone: true,
      className: "md:text-right",
      cell: (member) => (
        <Button variant="ghost" size="sm" onClick={() => onRemove(member)} aria-label={`Remove ${member.email} from ${scopeLabel(member)}`}>
          Remove
        </Button>
      ),
    },
  ];
  return <DataTable caption="People with access to this workspace" columns={columns} rows={sortMembers(members)} rowKey={(member) => member.id} />;
}
