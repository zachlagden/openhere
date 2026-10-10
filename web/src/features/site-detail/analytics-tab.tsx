import type { JSX } from "react";
import { AnalyticsPanel } from "@/features/analytics/analytics-panel";
import { canWrite } from "@/lib/roles";
import { useUpdateSite } from "@/queries/site-detail";
import { useSiteContext } from "./site-context";

export function AnalyticsTab(): JSX.Element {
  const { site, isRoot } = useSiteContext();
  const update = useUpdateSite(site.slug);
  return (
    <AnalyticsPanel
      scope={{ kind: "site", slug: site.slug }}
      isRoot={isRoot}
      recording={{
        enabled: site.analyticsEnabled,
        canChange: canWrite(site.role),
        pending: update.isPending,
        error: update.error,
        onEnable: () => update.mutate({ analyticsEnabled: true }),
      }}
    />
  );
}
