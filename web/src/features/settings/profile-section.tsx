import type { JSX } from "react";
import { Link } from "react-router";
import { Badge } from "@/components/ui/badge";
import { Section } from "@/components/ui/section";
import { formatDate } from "@/lib/format";
import { useInstance } from "@/queries/instance";
import { useMe } from "@/queries/session";

export function ProfileSection(): JSX.Element {
  const me = useMe();
  const instance = useInstance();
  const user = me.data;
  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <Section title="Profile" description="Your account on this server.">
        <dl className="grid gap-4 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium uppercase tracking-wider text-muted">Display name</dt>
            <dd className="mt-0.5">{user?.displayName ? user.displayName : <span className="text-muted">Not set</span>}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wider text-muted">Email address</dt>
            <dd className="mt-0.5 break-all">{user?.email}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wider text-muted">Role</dt>
            <dd className="mt-0.5">{user?.isRoot ? <Badge tone="accent">Root user</Badge> : "Member"}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wider text-muted">Account created</dt>
            <dd className="mt-0.5">{user ? formatDate(user.createdAt) : ""}</dd>
          </div>
        </dl>
        <p className="text-sm text-muted">The server has no way to change your display name or email address yet.</p>
      </Section>
      <Section title="Password" description="The server has no signed-in password change yet. Use a reset link instead.">
        <p className="text-sm">
          {instance.data?.emailAvailable ? (
            <>
              This server can send email, so you can{" "}
              <Link to="/reset-password" className="underline underline-offset-4">
                request a reset link
              </Link>{" "}
              for your address. The link works once and lasts one hour.
            </>
          ) : (
            <>This server cannot send email, so ask the root user for a reset link. The link works once and lasts one hour.</>
          )}
        </p>
      </Section>
    </div>
  );
}
