import type { FormEvent, JSX } from "react";
import { Navigate, useNavigate } from "react-router";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/field";
import { Spinner } from "@/components/ui/spinner";
import { useSetup, useSetupStatus } from "@/queries/session";
import { AuthLayout } from "./auth-layout";
import { FormError, useRateLimit } from "./form-error";
import { PasswordField } from "./password-field";

export function SetupPage(): JSX.Element {
  const status = useSetupStatus();
  const setup = useSetup();
  const navigate = useNavigate();
  const remaining = useRateLimit(setup.error);

  if (status.isPending) {
    return (
      <AuthLayout title="Set up">
        <Spinner label="Checking the server…" />
      </AuthLayout>
    );
  }
  if (status.data === false && !setup.isSuccess) {
    return <Navigate to="/login" replace />;
  }

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setup.mutate(
      {
        email: String(form.get("email") ?? ""),
        password: String(form.get("password") ?? ""),
        setupToken: String(form.get("setupToken") ?? "").trim(),
      },
      { onSuccess: () => void navigate("/", { replace: true }) },
    );
  }

  return (
    <AuthLayout title="Set up your server" description="Create the first account. It becomes the root user, who manages the whole instance.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Alert>
          <p className="font-medium">You need the setup token from the server log.</p>
          <p className="mt-1 text-muted">
            The server prints it once at startup, on the line that says &quot;setup required&quot;, in the field{" "}
            <code className="font-mono text-xs">setup_token</code>. With Docker, run{" "}
            <code className="font-mono text-xs">docker logs &lt;container&gt;</code> and look for that line.
          </p>
        </Alert>
        <TextField label="Setup token" name="setupToken" required autoComplete="off" spellCheck={false} className="font-mono" autoFocus />
        <TextField label="Email" name="email" type="email" autoComplete="username" required />
        <PasswordField label="Password" name="password" autoComplete="new-password" required minLength={12} maxLength={128} hint="12 to 128 characters." />
        <FormError error={setup.error} remaining={remaining} />
        <Button type="submit" variant="primary" disabled={setup.isPending || remaining > 0}>
          {setup.isPending ? "Creating account…" : "Create account"}
        </Button>
      </form>
    </AuthLayout>
  );
}
