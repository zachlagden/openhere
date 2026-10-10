import type { JSX } from "react";
import { AnalyticsPanel } from "@/features/analytics/analytics-panel";
import { Alert } from "@/components/ui/alert";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { useMe } from "@/queries/session";
import { useWorkspaces } from "@/queries/workspaces";

export function PersonalAnalyticsSection(): JSX.Element {
  const workspaces = useWorkspaces();
  const me = useMe();
  const personal = workspaces.data?.find((workspace) => workspace.kind === "personal");
  if (workspaces.isPending) {
    return <BlockSkeleton label="Loading workspace" rows={2} />;
  }
  if (workspaces.isError) {
    return <LoadError error={workspaces.error} onRetry={() => void workspaces.refetch()} />;
  }
  if (!personal) {
    return <Alert tone="info">You have no personal workspace.</Alert>;
  }
  return (
    <div className="flex flex-col gap-4">
      <p className="max-w-2xl text-sm text-muted">Combined page views for every site in your personal workspace.</p>
      <AnalyticsPanel scope={{ kind: "workspace", slug: personal.slug }} isRoot={me.data?.isRoot ?? false} />
    </div>
  );
}
