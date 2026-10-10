import type { FormEvent, JSX } from "react";
import { Link, Navigate, useSearchParams } from "react-router";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/field";
import { isApiPath, safeNextPath } from "@/lib/query-state";
import { useLogin, useMe, useSetupStatus } from "@/queries/session";
import { AuthLayout } from "./auth-layout";
import { FormError, useRateLimit } from "./form-error";
import { PasswordField } from "./password-field";
import { useAfterAuth } from "./use-after-auth";

export function LoginPage(): JSX.Element {
  const [params] = useSearchParams();
  const next = safeNextPath(params.get("next"));
  const me = useMe();
  const setupStatus = useSetupStatus();
  const login = useLogin();
  const afterAuth = useAfterAuth();
  const remaining = useRateLimit(login.error);

  if (setupStatus.data) {
    return <Navigate to="/setup" replace />;
  }
  if (me.data && !isApiPath(next)) {
    return <Navigate to={next} replace />;
  }

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    login.mutate(
      { email: String(form.get("email") ?? ""), password: String(form.get("password") ?? "") },
      { onSuccess: () => afterAuth(next) },
    );
  }

  return (
    <AuthLayout
      title="Sign in"
      description="Use the account for this server."
      footer={
        <p>
          <Link to="/reset-password" className="text-foreground underline underline-offset-4">
            Forgot your password?
          </Link>
        </p>
      }
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <TextField label="Email" name="email" type="email" autoComplete="username" required autoFocus />
        <PasswordField label="Password" name="password" autoComplete="current-password" required />
        <FormError error={login.error} remaining={remaining} />
        <Button type="submit" variant="primary" disabled={login.isPending || remaining > 0}>
          {login.isPending ? "Signing in…" : "Sign in"}
        </Button>
      </form>
    </AuthLayout>
  );
}
