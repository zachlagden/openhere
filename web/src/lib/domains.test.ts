import { describe, expect, it } from "vitest";
import type { Domain } from "@/api/types";
import { challengeRecord, cleanDomainInput, describeCheckResult, domainSlotsLeft, domainStatus, targetRecord } from "./domains";

const domain: Domain = {
  id: "d1",
  domain: "docs.example.org",
  status: "pending_verification",
  challenge: { recordType: "TXT", name: "_openhere-challenge.docs.example.org", value: "ab12cd34" },
  target: { recordType: "CNAME", value: "sites.example.com", note: "Use an ALIAS record on an apex domain." },
  isPrimary: false,
  verifiedAt: null,
  lastCheckedAt: null,
  lastCheckResult: null,
  createdAt: "2026-03-01T10:00:00Z",
};

describe("domain display helpers", () => {
  it("labels the status", () => {
    expect(domainStatus(domain)).toEqual({ label: "Waiting for DNS", tone: "warn" });
    expect(domainStatus({ ...domain, status: "verified" })).toEqual({ label: "Verified", tone: "ok" });
  });

  it("describes every check result in plain language", () => {
    expect(describeCheckResult(null)).toBe("Not checked yet.");
    expect(describeCheckResult("record_not_found")).toMatch(/No TXT record was found/);
    expect(describeCheckResult("token_mismatch")).toMatch(/does not match/);
    expect(describeCheckResult("lookup_failed")).toMatch(/says nothing about your record/);
    expect(describeCheckResult("verified")).toMatch(/matched/);
  });

  it("builds the TXT challenge and the pointing record", () => {
    expect(challengeRecord(domain)).toMatchObject({ type: "TXT", name: "_openhere-challenge.docs.example.org", value: "ab12cd34" });
    expect(targetRecord(domain)).toMatchObject({ type: "CNAME", name: "docs.example.org", value: "sites.example.com" });
  });

  it("cleans input", () => {
    expect(cleanDomainInput("  Docs.Example.ORG. ")).toBe("docs.example.org");
  });

  it("counts free slots", () => {
    expect(domainSlotsLeft(3, 10)).toBe(7);
    expect(domainSlotsLeft(12, 10)).toBe(0);
  });
});
