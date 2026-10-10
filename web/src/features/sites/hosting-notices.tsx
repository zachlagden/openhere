import { AlertTriangle, Info } from "lucide-react";
import type { JSX } from "react";
import { Link } from "react-router";
import type { SiteHosting } from "@/api/types";
import { cn } from "@/lib/cn";

export type NoticeSeverity = "warning" | "caution" | "info";

export function noticeSeverity(hosting: SiteHosting): NoticeSeverity {
  if (!hosting.reachable) {
    return "warning";
  }
  const modes = Object.values(hosting.modes);
  return modes.some((mode) => mode.state === "off") ? "caution" : "info";
}

const severityClasses: Record<NoticeSeverity, string> = {
  warning: "rounded-md bg-warn-soft px-3 py-2 text-warn",
  caution: "text-warn",
  info: "text-muted",
};

interface HostingNoticesProps {
  hosting: SiteHosting;
  isRoot: boolean;
}

export function HostingNotices({ hosting, isRoot }: HostingNoticesProps): JSX.Element | null {
  const severity = noticeSeverity(hosting);
  const messages = hosting.notices.map((notice) => notice.message);
  if (messages.length === 0) {
    return null;
  }
  const Icon = severity === "info" ? Info : AlertTriangle;
  return (
    <div className={cn("flex items-start gap-2 text-sm", severityClasses[severity])} data-severity={severity}>
      <Icon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <div className="min-w-0 flex-1">
        <ul>
          {messages.map((message) => (
            <li key={message}>{message}</li>
          ))}
        </ul>
        {isRoot && severity !== "info" ? (
          <Link to="/settings/hosting" className="mt-0.5 inline-block font-medium underline underline-offset-4">
            Open hosting settings
          </Link>
        ) : null}
      </div>
    </div>
  );
}
