import { useState, type JSX } from "react";
import { describeError } from "@/api/errors";
import type { Access } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { useToast } from "@/components/ui/toast";
import { PasswordField } from "@/features/auth/password-field";
import { MAX_PASSWORD_LENGTH, MIN_PASSWORD_LENGTH } from "@/lib/access-form";
import { useUpdateAccess } from "@/queries/site-detail";

interface PasswordControlsProps {
  slug: string;
  access: Access;
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
}

export function PasswordControls({ slug, access, value, onChange, disabled }: PasswordControlsProps): JSX.Element {
  const [confirming, setConfirming] = useState(false);
  const clear = useUpdateAccess(slug);
  const toast = useToast();
  const inUse = access.mode === "password";

  function confirm(): void {
    clear.mutate(
      { password: null },
      {
        onSuccess: () => {
          setConfirming(false);
          toast.success("Password removed.");
        },
      },
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="text-sm font-medium">Password</h3>
        <Badge tone={access.hasPassword ? "ok" : "neutral"}>{access.hasPassword ? "A password is set" : "No password set"}</Badge>
      </div>
      <p className="text-sm text-muted">The password is stored as a hash and is never shown again. Setting a new one replaces the old one and signs every visitor out.</p>
      {disabled ? null : (
        <div className="flex flex-col gap-3 sm:max-w-sm">
          <PasswordField
            label={access.hasPassword ? "New password" : "Password"}
            value={value}
            onChange={(event) => onChange(event.target.value)}
            autoComplete="new-password"
            minLength={MIN_PASSWORD_LENGTH}
            maxLength={MAX_PASSWORD_LENGTH}
            hint={`${MIN_PASSWORD_LENGTH} to ${MAX_PASSWORD_LENGTH} characters. Saved with the other changes.`}
          />
          {access.hasPassword ? (
            <div>
              <Button variant="secondary" size="sm" disabled={inUse} onClick={() => setConfirming(true)}>
                Remove password
              </Button>
              {inUse ? <p className="mt-1 text-xs text-muted">Switch to another mode and save before removing the password.</p> : null}
            </div>
          ) : null}
        </div>
      )}
      <ConfirmDialog
        open={confirming}
        onOpenChange={(open) => {
          if (!open) {
            clear.reset();
          }
          setConfirming(open);
        }}
        title="Remove the password?"
        confirmLabel="Remove password"
        pending={clear.isPending}
        error={clear.error ? describeError(clear.error).message : null}
        onConfirm={confirm}
      >
        <p>The stored password is deleted and visitors who knew it are signed out. You can set a new one later.</p>
      </ConfirmDialog>
    </div>
  );
}
