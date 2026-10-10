import { BarChart3, Globe, Lock, ShieldCheck } from "lucide-react";
import type { JSX } from "react";
import type { AccessMode, Site } from "@/api/types";
import { Badge } from "@/components/ui/badge";

const ACCESS: Record<AccessMode, { label: string; icon: JSX.Element; title: string }> = {
  public: { label: "Public", icon: <Globe aria-hidden="true" />, title: "Anyone with the link can open this site" },
  password: { label: "Password", icon: <Lock aria-hidden="true" />, title: "Visitors need the site password" },
  restricted: { label: "Restricted", icon: <ShieldCheck aria-hidden="true" />, title: "Only people on the access list can open this site" },
};

export function AccessBadge({ mode }: { mode: AccessMode }): JSX.Element {
  const { label, icon, title } = ACCESS[mode];
  return (
    <Badge tone={mode === "public" ? "neutral" : "accent"} title={title}>
      {icon}
      {label}
    </Badge>
  );
}

export function AnalyticsBadge({ enabled }: { enabled: boolean }): JSX.Element {
  return (
    <Badge title={enabled ? "Page views are counted" : "Page views are not counted"} className={enabled ? undefined : "opacity-70"}>
      <BarChart3 aria-hidden="true" />
      {enabled ? "Analytics on" : "Analytics off"}
    </Badge>
  );
}

export function PublishBadge({ site }: { site: Site }): JSX.Element {
  if (site.currentVersionId === null) {
    return <Badge title="Nothing has been published to this site yet">Not published</Badge>;
  }
  if (site.hosting.reachable) {
    return (
      <Badge tone="ok" title="The site is live and reachable">
        Live
      </Badge>
    );
  }
  return (
    <Badge tone="warn" title="The site is published but cannot be opened at any address right now">
      Unreachable
    </Badge>
  );
}
