import type { Domain, Site } from "@/api/types";

export interface DomainRow {
  site: Site;
  domain: Domain;
}

export function toRows(entries: { site: Site; domains: Domain[] }[]): DomainRow[] {
  const rows = entries.flatMap(({ site, domains }) => domains.map((domain) => ({ site, domain })));
  return rows.sort((left, right) => {
    const waiting = Number(left.domain.status === "verified") - Number(right.domain.status === "verified");
    return waiting || left.domain.domain.localeCompare(right.domain.domain);
  });
}

export function countWaiting(rows: DomainRow[]): number {
  return rows.filter((row) => row.domain.status !== "verified").length;
}
