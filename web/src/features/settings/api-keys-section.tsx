import { KeyRound, Plus } from "lucide-react";
import { useState, type JSX } from "react";
import type { ApiKey } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DataTable, type Column } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import { SecretDialog } from "@/components/ui/secret-dialog";
import { useToast } from "@/components/ui/toast";
import { MAX_ACTIVE_KEYS, activeKeyCount, describeKeyError, isActive, keyStatus } from "@/lib/api-keys";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { useApiKeys, useRevokeApiKey } from "@/queries/api-keys";
import { CreateKeyDialog } from "./create-key-dialog";

function KeysTable({ keys, onRevoke }: { keys: ApiKey[]; onRevoke: (key: ApiKey) => void }): JSX.Element {
  const columns: Column<ApiKey>[] = [
    { key: "label", header: "Label", cell: (key) => <span className="break-words font-medium">{key.label}</span> },
    { key: "prefix", header: "Key", cell: (key) => <code className="font-mono text-[0.8125rem]">{key.prefix}…</code> },
    {
      key: "created",
      header: "Created",
      cell: (key) => (
        <time dateTime={key.createdAt} title={formatDateTime(key.createdAt)}>
          {formatRelativeTime(key.createdAt)}
        </time>
      ),
    },
    {
      key: "used",
      header: "Last used",
      cell: (key) =>
        key.lastUsedAt ? (
          <time dateTime={key.lastUsedAt} title={formatDateTime(key.lastUsedAt)}>
            {formatRelativeTime(key.lastUsedAt)}
          </time>
        ) : (
          "Never"
        ),
    },
    {
      key: "status",
      header: "Status",
      cell: (key) => {
        const status = keyStatus(key);
        return <Badge tone={status.tone}>{status.label}</Badge>;
      },
    },
    {
      key: "actions",
      header: "Actions",
      hideLabelOnPhone: true,
      className: "md:text-right",
      cell: (key) =>
        isActive(key) ? (
          <Button variant="ghost" size="sm" onClick={() => onRevoke(key)} aria-label={`Revoke key ${key.label}`}>
            Revoke
          </Button>
        ) : null,
    },
  ];
  return <DataTable caption="Your API keys" columns={columns} rows={keys} rowKey={(key) => key.id} />;
}

export function ApiKeysSection(): JSX.Element {
  const keys = useApiKeys();
  const revoke = useRevokeApiKey();
  const toast = useToast();
  const [creating, setCreating] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<ApiKey | null>(null);
  const active = keys.data ? activeKeyCount(keys.data) : 0;
  const atLimit = active >= MAX_ACTIVE_KEYS;

  function confirmRevoke(): void {
    if (!revoking) {
      return;
    }
    const target = revoking;
    revoke.mutate(target.id, {
      onSuccess: () => {
        setRevoking(null);
        toast.success(`Revoked ${target.label}.`);
      },
    });
  }

  return (
    <div className="flex max-w-5xl flex-col gap-6">
      <Section
        title="API keys"
        description="Agents and scripts use a key to publish and manage sites through the API. A key has the full access of your account."
        actions={
          <Button variant="primary" disabled={atLimit} onClick={() => setCreating(true)}>
            <Plus /> Create key
          </Button>
        }
      >
        <p className="text-sm text-muted">
          {active} of {MAX_ACTIVE_KEYS} active keys used.
          {atLimit ? " Revoke a key you no longer use to create another." : ""}
        </p>
        {keys.isPending ? <BlockSkeleton label="Loading API keys" rows={3} /> : null}
        {keys.isError ? <LoadError error={keys.error} onRetry={() => void keys.refetch()} /> : null}
        {keys.data && keys.data.length === 0 ? (
          <EmptyState icon={<KeyRound />} title="No API keys yet">
            <p>Create a key when an agent or script needs to call the API. The full key is shown once, when you create it.</p>
          </EmptyState>
        ) : null}
        {keys.data && keys.data.length > 0 ? <KeysTable keys={keys.data} onRevoke={setRevoking} /> : null}
      </Section>
      <CreateKeyDialog open={creating} onOpenChange={setCreating} onCreated={setSecret} />
      <SecretDialog
        title="Your new API key"
        fieldLabel="API key"
        secret={secret}
        warning="This is the only time the full key is shown. Copy it now and keep it somewhere safe. Anyone with the key has the full access of your account."
        onClose={() => setSecret(null)}
      />
      <ConfirmDialog
        open={revoking !== null}
        onOpenChange={(open) => {
          if (!open) {
            setRevoking(null);
            revoke.reset();
          }
        }}
        title={revoking ? `Revoke ${revoking.label}?` : "Revoke key"}
        confirmLabel="Revoke key"
        pending={revoke.isPending}
        error={revoke.error ? describeKeyError(revoke.error) : null}
        onConfirm={confirmRevoke}
      >
        <p>Anything that uses this key stops working at once. This cannot be undone.</p>
      </ConfirmDialog>
    </div>
  );
}
