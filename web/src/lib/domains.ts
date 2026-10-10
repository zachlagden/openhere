import type { Domain } from "@/api/types";

export type DomainTone = "ok" | "warn";

export function domainStatus(domain: Domain): { label: string; tone: DomainTone } {
  return domain.status === "verified" ? { label: "Verified", tone: "ok" } : { label: "Waiting for DNS", tone: "warn" };
}

const CHECK_MESSAGES: Record<NonNullable<Domain["lastCheckResult"]>, string> = {
  verified: "The TXT record matched.",
  record_not_found: "No TXT record was found. DNS changes can take a while to spread, so try again in a few minutes.",
  token_mismatch: "A TXT record exists, but its value does not match. Check that you copied the value exactly.",
  lookup_failed: "The DNS lookup failed or timed out. That says nothing about your record. Try again shortly.",
};

export function describeCheckResult(result: Domain["lastCheckResult"]): string {
  return result === null ? "Not checked yet." : CHECK_MESSAGES[result];
}

export interface DnsRecordRow {
  purpose: string;
  type: string;
  name: string;
  value: string;
}

export function challengeRecord(domain: Domain): DnsRecordRow {
  return { purpose: "Proves you control the domain", type: domain.challenge.recordType, name: domain.challenge.name, value: domain.challenge.value };
}

export function targetRecord(domain: Domain): DnsRecordRow {
  return { purpose: "Sends visitors to this server", type: domain.target.recordType, name: domain.domain, value: domain.target.value };
}

export function cleanDomainInput(raw: string): string {
  return raw.trim().toLowerCase().replace(/\.$/, "");
}

export function domainSlotsLeft(count: number, limit: number): number {
  return Math.max(0, limit - count);
}
