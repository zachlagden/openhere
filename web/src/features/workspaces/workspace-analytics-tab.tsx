import type { JSX } from "react";
import { Alert } from "@/components/ui/alert";
import { AnalyticsPanel } from "@/features/analytics/analytics-panel";
import { canSeeWorkspaceAnalytics } from "@/lib/workspace-access";
import { useWorkspaceContext } from "./workspace-context";

export function WorkspaceAnalyticsTab(): JSX.Element {
  const { workspace, isRoot } = useWorkspaceContext();
  if (!canSeeWorkspaceAnalytics(workspace)) {
    return (
      <Alert tone="info">
        <p className="font-medium">Workspace analytics is not available to you.</p>
        <p className="mt-1">
          Combined numbers need access to the whole workspace, and you can reach only some folders and sites here. Open a site you can reach to see its analytics.
        </p>
      </Alert>
    );
  }
  return (
    <div className="flex flex-col gap-4">
      <p className="max-w-2xl text-sm text-muted">
        {workspace.kind === "personal"
          ? "Combined page views for every site in your personal workspace."
          : "Combined page views for the sites in this workspace that you can see. Unique visitors are added up per site, so a visitor who opens two sites counts twice."}
      </p>
      <AnalyticsPanel scope={{ kind: "workspace", slug: workspace.slug }} isRoot={isRoot} />
    </div>
  );
}
