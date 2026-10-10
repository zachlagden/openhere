import type { FormEvent, JSX } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { isApiError } from "@/api/client";
import { describeError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/field";
import { Spinner } from "@/components/ui/spinner";
import { useAcceptInvite, useInvite } from "@/queries/account";
import { AuthLayout } from "./auth-layout";
import { FormError, useRateLimit } from "./form-error";
import { PasswordField } from "./password-field";

export function InvitePage(): JSX.Element {
  const { token = "" } = useParams();
  const invite = useInvite(token);
  const accept = useAcceptInvite(token);
  const navigate = useNavigate();
  const remaining = useRateLimit(accept.error);

  if (invite.isPending) {
    return (
      <AuthLayout title="Your invite">
        <Spinner label="Checking the invite…" />
      </AuthLayout>
    );
  }
  if (invite.isError) {
    const invalid = isApiError(invite.error) && invite.error.status === 404;
    return (
      <AuthLayout title={invalid ? "This invite is not valid" : "Could not load the invite"}>
        <p className="text-sm text-muted">
          {invalid
            ? "The link may have expired, been revoked or been used already. Ask the person who invited you for a new link."
            : describeError(invite.error).message}
        </p>
        <div className="mt-5">
          <Button asChild variant="secondary">
            <Link to="/login">Go to sign in</Link>
          </Button>
        </div>
      </AuthLayout>
    );
  }

  const { email, workspaceName } = invite.data;

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const displayName = String(form.get("displayName") ?? "").trim();
    accept.mutate(
      { password: String(form.get("password") ?? ""), ...(displayName === "" ? {} : { displayName }) },
      { onSuccess: () => void navigate("/", { replace: true }) },
    );
  }

  return (
    <AuthLayout
      title={workspaceName ? `Join ${workspaceName}` : "Create your account"}
      description={workspaceName ? "Choose a password to create your account and join the workspace." : "Choose a password to create your account."}
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Alert>
          <p>
            You are invited as <span className="font-medium">{email}</span>. You cannot change this address.
          </p>
        </Alert>
        <TextField label="Name (optional)" name="displayName" autoComplete="name" maxLength={64} />
        <PasswordField label="Password" name="password" autoComplete="new-password" required minLength={12} maxLength={128} hint="12 to 128 characters." autoFocus />
        <FormError error={accept.error} remaining={remaining} />
        <Button type="submit" variant="primary" disabled={accept.isPending || remaining > 0}>
          {accept.isPending ? "Creating account…" : "Create account"}
        </Button>
      </form>
    </AuthLayout>
  );
}
