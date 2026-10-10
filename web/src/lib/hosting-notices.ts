import type { Site, SiteHosting } from "@/api/types";

export type NoticeSeverity = "warning" | "info";

type HostingSubject = Pick<Site, "hosting" | "currentVersionId">;

export function modeSwitchedOffBySystem(hosting: SiteHosting): boolean {
  return Object.values(hosting.modes).some((mode) => mode.reason === "wildcard_check_failed");
}

export function noticeSeverity(site: HostingSubject): NoticeSeverity {
  const unreachable = site.currentVersionId !== null && !site.hosting.reachable;
  return unreachable || modeSwitchedOffBySystem(site.hosting) ? "warning" : "info";
}

export function noticeMessages(site: HostingSubject, variant: "list" | "detail"): string[] {
  if (variant === "list" && noticeSeverity(site) === "info") {
    return [];
  }
  return site.hosting.notices.map((notice) => notice.message);
}
