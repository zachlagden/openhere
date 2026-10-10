import { useState, type FormEvent, type JSX } from "react";
import type { CreatedInvite, Role } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { SelectField, TextField } from "@/components/ui/field";
import { SwitchField } from "@/components/ui/switch";
import { describeInviteError } from "@/lib/invites";
import { roleLabel } from "@/lib/roles";
import { useCreateInvite } from "@/queries/invites";
import { useInstance } from "@/queries/instance";
import { useWorkspaces } from "@/queries/workspaces";

const NO_WORKSPACE = "";

function isRole(value: string): value is Role {
  return value === "admin" || value === "write" || value === "read";
}

export function InviteUserForm({ onCreated }: { onCreated: (created: CreatedInvite, askedForEmail: boolean) => void }): JSX.Element {
  const create = useCreateInvite();
  const instance = useInstance();
  const workspaces = useWorkspaces();
  const [email, setEmail] = useState("");
  const [workspace, setWorkspace] = useState(NO_WORKSPACE);
  const [role, setRole] = useState<Role>("read");
  const [sendEmail, setSendEmail] = useState(false);
  const emailAvailable = instance.data?.emailAvailable ?? false;
  const shared = (workspaces.data ?? []).filter((item) => item.kind === "shared");

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const askedForEmail = emailAvailable && sendEmail;
    create.mutate(
      { email: email.trim(), sendEmail: askedForEmail, ...(workspace === NO_WORKSPACE ? {} : { workspace, role }) },
      {
        onSuccess: (created) => {
          setEmail("");
          setSendEmail(false);
          onCreated(created, askedForEmail);
          create.reset();
        },
      },
    );
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4" aria-label="Invite a user">
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField label="Email address" type="email" required value={email} onChange={(event) => setEmail(event.target.value)} placeholder="kim@example.com" autoComplete="off" hint="The invite only works for this address. It lasts 7 days." />
        <SelectField label="Workspace" value={workspace} onChange={(event) => setWorkspace(event.target.value)} hint="Optional. Without one, the person gets an account and a personal workspace only.">
          <option value={NO_WORKSPACE}>No workspace</option>
          {shared.map((item) => (
            <option key={item.id} value={item.slug}>
              {item.name}
            </option>
          ))}
        </SelectField>
        {workspace === NO_WORKSPACE ? null : (
          <SelectField label="Role in that workspace" value={role} onChange={(event) => isRole(event.target.value) && setRole(event.target.value)}>
            {(["read", "write", "admin"] as const).map((item) => (
              <option key={item} value={item}>
                {roleLabel(item)}
              </option>
            ))}
          </SelectField>
        )}
      </div>
      {emailAvailable ? (
        <SwitchField label="Email the link too" description="The link is shown to you either way." checked={sendEmail} onChange={(event) => setSendEmail(event.target.checked)} />
      ) : (
        <p className="text-sm text-muted">This server cannot send email. After you create the invite you get a link to pass on yourself.</p>
      )}
      {create.isError ? <Alert tone="error">{describeInviteError(create.error)}</Alert> : null}
      <div>
        <Button type="submit" variant="primary" disabled={create.isPending || email.trim() === ""}>
          {create.isPending ? "Creating…" : "Create invite"}
        </Button>
      </div>
    </form>
  );
}
