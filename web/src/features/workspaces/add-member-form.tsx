import { useState, type FormEvent, type JSX } from "react";
import type { Role } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { SelectField, TextField } from "@/components/ui/field";
import { useToast } from "@/components/ui/toast";
import { buildAddRequest, describeMemberError, roleOptions, validateAddMember, type AddMemberForm, type ScopeType } from "@/lib/members";
import { roleLabel } from "@/lib/roles";
import { useFolders } from "@/queries/folders";
import { useAddMember } from "@/queries/members";
import { useWorkspaceSiteChoices } from "@/queries/sites";

const EMPTY: AddMemberForm = { email: "", role: "read", scopeType: "workspace", scopeId: "" };

function isRole(value: string): value is Role {
  return value === "admin" || value === "write" || value === "read";
}

function isScope(value: string): value is ScopeType {
  return value === "workspace" || value === "folder" || value === "site";
}

export function AddMemberFormSection({ workspace }: { workspace: string }): JSX.Element {
  const add = useAddMember(workspace);
  const toast = useToast();
  const [form, setForm] = useState<AddMemberForm>(EMPTY);
  const [problem, setProblem] = useState<string | null>(null);
  const folders = useFolders(form.scopeType === "folder" ? workspace : undefined);
  const sites = useWorkspaceSiteChoices(workspace, form.scopeType === "site");

  function changeScope(value: string): void {
    if (!isScope(value)) {
      return;
    }
    const role = value !== "workspace" && form.role === "admin" ? "write" : form.role;
    setForm({ ...form, scopeType: value, scopeId: "", role });
  }

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const invalid = validateAddMember(form);
    setProblem(invalid);
    if (invalid !== null) {
      return;
    }
    add.mutate(buildAddRequest(form), {
      onSuccess: (member) => {
        setForm(EMPTY);
        toast.success(`Added ${member.email}.`);
      },
    });
  }

  const targets = form.scopeType === "folder" ? (folders.data ?? []).map((item) => ({ id: item.id, name: item.name })) : (sites.data ?? []).map((item) => ({ id: item.slug, name: item.displayName }));

  return (
    <form onSubmit={submit} className="flex flex-col gap-4" aria-label="Add a member">
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField
          label="Email address"
          type="email"
          value={form.email}
          onChange={(event) => setForm({ ...form, email: event.target.value })}
          placeholder="sam@example.com"
          hint="The person must already have an account on this server."
          autoComplete="off"
        />
        <SelectField label="Role" value={form.role} onChange={(event) => isRole(event.target.value) && setForm({ ...form, role: event.target.value })}>
          {roleOptions(form.scopeType).map((role) => (
            <option key={role} value={role}>
              {roleLabel(role)}
            </option>
          ))}
        </SelectField>
        <SelectField label="Access to" value={form.scopeType} onChange={(event) => changeScope(event.target.value)}>
          <option value="workspace">The whole workspace</option>
          <option value="folder">One folder</option>
          <option value="site">One site</option>
        </SelectField>
        {form.scopeType === "workspace" ? null : (
          <SelectField
            label={form.scopeType === "folder" ? "Folder" : "Site"}
            value={form.scopeId}
            onChange={(event) => setForm({ ...form, scopeId: event.target.value })}
            disabled={folders.isPending && sites.isPending}
          >
            <option value="">Choose…</option>
            {targets.map((target) => (
              <option key={target.id} value={target.id}>
                {target.name}
              </option>
            ))}
          </SelectField>
        )}
      </div>
      {problem ? <Alert tone="warning">{problem}</Alert> : null}
      {add.isError ? <Alert tone="error">{describeMemberError(add.error)}</Alert> : null}
      <div>
        <Button type="submit" variant="primary" disabled={add.isPending}>
          {add.isPending ? "Adding…" : "Add member"}
        </Button>
      </div>
    </form>
  );
}
