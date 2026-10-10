import { useState, type FormEvent, type JSX } from "react";
import { Link, useParams } from "react-router";
import { isApiError } from "@/api/client";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { SwitchField } from "@/components/ui/switch";
import { useConfirmPasswordReset } from "@/queries/account";
import { AuthLayout } from "./auth-layout";
import { FormError, useRateLimit } from "./form-error";
import { PasswordField } from "./password-field";

export function ResetConfirmPage(): JSX.Element {
  const { token = "" } = useParams();
  const confirm = useConfirmPasswordReset();
  const remaining = useRateLimit(confirm.error);
  const [mismatch, setMismatch] = useState(false);

  if (confirm.isSuccess) {
    return (
      <AuthLayout title="Password changed" description="Your other sessions have ended. Sign in with the new password.">
        <Button asChild variant="primary">
          <Link to="/login">Go to sign in</Link>
        </Button>
      </AuthLayout>
    );
  }

  const invalidLink = isApiError(confirm.error) && confirm.error.code === "invalid_reset_token";

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") ?? "");
    if (password !== String(form.get("confirm") ?? "")) {
      setMismatch(true);
      return;
    }
    setMismatch(false);
    confirm.mutate({ token, password, revokeApiKeys: form.get("revokeApiKeys") === "on" });
  }

  return (
    <AuthLayout title="Choose a new password">
      {invalidLink ? (
        <div className="flex flex-col gap-4">
          <Alert tone="error">This reset link is not valid. It may have expired or been used. Request a new one.</Alert>
          <Button asChild variant="primary">
            <Link to="/reset-password">Request a new link</Link>
          </Button>
        </div>
      ) : (
        <form onSubmit={submit} className="flex flex-col gap-4">
          <PasswordField label="New password" name="password" autoComplete="new-password" required minLength={12} maxLength={128} hint="12 to 128 characters." autoFocus />
          <PasswordField label="Repeat the new password" name="confirm" autoComplete="new-password" required />
          <SwitchField
            name="revokeApiKeys"
            label="Also revoke my API keys"
            description="Turn this on if someone else may have a key. Scripts and agents that use your keys will stop working."
          />
          {mismatch ? <Alert tone="error">The two passwords do not match.</Alert> : null}
          <FormError error={confirm.error} remaining={remaining} />
          <Button type="submit" variant="primary" disabled={confirm.isPending || remaining > 0}>
            {confirm.isPending ? "Saving…" : "Change password"}
          </Button>
        </form>
      )}
    </AuthLayout>
  );
}
