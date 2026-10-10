import { useState, type FormEvent, type JSX } from "react";
import { Link } from "react-router";
import type { Access } from "@/api/types";
import { Alert } from "@/components/ui/alert";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { SaveBar } from "@/components/ui/save-bar";
import { Section } from "@/components/ui/section";
import { useToast } from "@/components/ui/toast";
import { FormError } from "@/features/auth/form-error";
import { accessProblem, buildAccessPatch, draftFromAccess, type AccessDraft } from "@/lib/access-form";
import { canWrite } from "@/lib/roles";
import { useAccess, useUpdateAccess } from "@/queries/site-detail";
import { AccessModePicker } from "./access-mode-picker";
import { AllowlistEditor } from "./allowlist-editor";
import { PasswordControls } from "./password-controls";
import { useSiteContext } from "./site-context";

function EmailCodeNote({ available, isRoot }: { available: boolean; isRoot: boolean }): JSX.Element | null {
  if (available) {
    return null;
  }
  return (
    <Alert tone="warning">
      <p>
        This server cannot send email yet, so visitors on the lists cannot get a sign-in code. The entries are saved, and users of this server can still sign in with their account.
      </p>
      {isRoot ? (
        <Link to="/admin/email" className="mt-1 inline-block font-medium underline underline-offset-4">
          Set up email
        </Link>
      ) : (
        <p className="mt-1">Ask the server administrator to set up email.</p>
      )}
    </Alert>
  );
}

interface AccessFormProps {
  slug: string;
  access: Access;
  editable: boolean;
  isRoot: boolean;
}

function AccessForm({ slug, access, editable, isRoot }: AccessFormProps): JSX.Element {
  const update = useUpdateAccess(slug);
  const toast = useToast();
  const [draft, setDraft] = useState<AccessDraft>(() => draftFromAccess(access));
  const patch = buildAccessPatch(access, draft);
  const dirty = Object.keys(patch).length > 0;
  const problem = dirty ? accessProblem(access, draft) : null;

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    if (problem === null) {
      update.mutate(patch, {
        onSuccess: () => {
          setDraft((current) => ({ ...current, password: "" }));
          toast.success("Access settings saved.");
        },
      });
    }
  }

  return (
    <form onSubmit={submit} className="flex max-w-3xl flex-col gap-6">
      <Section title="Mode" description="Changing the mode, the password or a list signs every visitor out.">
        <AccessModePicker value={draft.mode} onChange={(mode) => setDraft({ ...draft, mode })} disabled={!editable} />
      </Section>
      <Section title="Password" description="Used when the mode is password.">
        <PasswordControls slug={slug} access={access} value={draft.password} onChange={(password) => setDraft({ ...draft, password })} disabled={!editable} />
      </Section>
      <Section
        title="Allowed visitors"
        description="Used when the mode is restricted. On the sign-in page, a listed visitor enters their email address and receives an 8-digit code. Users of this server choose the account sign-in link instead."
      >
        <EmailCodeNote available={access.emailCodeAvailable} isRoot={isRoot} />
        <AllowlistEditor kind="email" entries={draft.emailAllowlist} onChange={(emailAllowlist) => setDraft({ ...draft, emailAllowlist })} disabled={!editable} />
        <AllowlistEditor kind="domain" entries={draft.domainAllowlist} onChange={(domainAllowlist) => setDraft({ ...draft, domainAllowlist })} disabled={!editable} />
      </Section>
      {editable ? (
        <div className="flex flex-col gap-3">
          {problem ? <Alert tone="warning">{problem}</Alert> : null}
          <FormError error={update.error} />
          <SaveBar
            dirty={dirty && problem === null}
            pending={update.isPending}
            onDiscard={() => {
              setDraft(draftFromAccess(access));
              update.reset();
            }}
          />
        </div>
      ) : (
        <p className="text-sm text-muted">You have read-only access to this site, so you cannot change who can open it.</p>
      )}
    </form>
  );
}

export function AccessTab(): JSX.Element {
  const { site, isRoot } = useSiteContext();
  const access = useAccess(site.slug);
  if (access.isPending) {
    return <BlockSkeleton label="Loading access settings" />;
  }
  if (access.isError) {
    return <LoadError error={access.error} onRetry={() => void access.refetch()} />;
  }
  return <AccessForm slug={site.slug} access={access.data} editable={canWrite(site.role)} isRoot={isRoot} />;
}
