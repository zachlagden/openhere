import { AlertTriangle, Info } from "lucide-react";
import type { JSX, ReactNode } from "react";
import { Link } from "react-router";
import type { Site } from "@/api/types";
import { cn } from "@/lib/cn";
import { noticeMessages, noticeSeverity, type NoticeSeverity } from "@/lib/hosting-notices";

const severityClasses: Record<NoticeSeverity, string> = {
  warning: "rounded-md bg-warn-soft px-3 py-2 text-warn",
  info: "text-muted",
};

interface HostingNoticesProps {
  site: Pick<Site, "hosting" | "currentVersionId">;
  isRoot: boolean;
  variant: "list" | "detail";
  cause?: ReactNode;
}

export function HostingNotices({ site, isRoot, variant, cause }: HostingNoticesProps): JSX.Element | null {
  const severity = noticeSeverity(site);
  const messages = noticeMessages(site, variant);
  if (messages.length === 0) {
    return null;
  }
  const Icon = severity === "info" ? Info : AlertTriangle;
  const showRootDetail = isRoot && (variant === "detail" || severity === "warning");
  return (
    <div className={cn("flex items-start gap-2 text-sm", severityClasses[severity])} data-severity={severity}>
      <Icon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <div className="min-w-0 flex-1">
        <ul>
          {messages.map((message) => (
            <li key={message}>{message}</li>
          ))}
        </ul>
        {showRootDetail && cause ? <div className="mt-1 text-xs">{cause}</div> : null}
        {showRootDetail ? (
          <Link to="/admin/hosting" className="mt-0.5 inline-block font-medium underline underline-offset-4">
            Open hosting settings
          </Link>
        ) : null}
      </div>
    </div>
  );
}
