import type { FormEvent, JSX } from "react";
import { Link } from "react-router";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/field";
import { useRequestPasswordReset } from "@/queries/account";
import { AuthLayout } from "./auth-layout";
import { FormError, useRateLimit } from "./form-error";

export function ResetRequestPage(): JSX.Element {
  const reset = useRequestPasswordReset();
  const remaining = useRateLimit(reset.error);

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    reset.mutate(String(form.get("email") ?? ""));
  }

  return (
    <AuthLayout
      title="Reset your password"
      description="Enter your email address. If an account exists and this server can send email, you get a link that works for one hour."
      footer={
        <p>
          <Link to="/login" className="text-foreground underline underline-offset-4">
            Back to sign in
          </Link>
        </p>
      }
    >
      {reset.isSuccess ? (
        <Alert>
          <p className="font-medium">Check your email.</p>
          <p className="mt-1 text-muted">
            If the address belongs to an account, a reset link is on its way. If nothing arrives, this server may not send email. Ask an
            administrator for a reset link.
          </p>
        </Alert>
      ) : (
        <form onSubmit={submit} className="flex flex-col gap-4">
          <TextField label="Email" name="email" type="email" autoComplete="username" required autoFocus />
          <FormError error={reset.error} remaining={remaining} />
          <Button type="submit" variant="primary" disabled={reset.isPending || remaining > 0}>
            {reset.isPending ? "Sending…" : "Send reset link"}
          </Button>
        </form>
      )}
    </AuthLayout>
  );
}
