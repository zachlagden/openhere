import type { JSX } from "react";
import { useHostingStatus } from "@/queries/admin";

export function HostingCause(): JSX.Element | null {
  const status = useHostingStatus(true);
  const detail = status.data?.detail;
  if (!detail) {
    return null;
  }
  const parts: string[] = [];
  if (detail.subdomainDisabledReason === "wildcard_check_failed") {
    parts.push("Subdomain mode was switched off after repeated failed wildcard checks.");
  }
  if (detail.wildcardStatus === "failed") {
    parts.push(detail.wildcardDetail === "" ? "The last wildcard check failed." : `The last wildcard check failed: ${detail.wildcardDetail}`);
  }
  if (parts.length === 0) {
    return null;
  }
  return <p>{parts.join(" ")}</p>;
}
