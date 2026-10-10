import { describe, expect, it } from "vitest";
import { makeDomain, makeSite } from "@/test/fixtures";
import { countWaiting, toRows } from "./domains-overview";

describe("domain overview rows", () => {
  const site = makeSite();
  const rows = toRows([
    { site, domains: [makeDomain({ id: "a", domain: "b.example.org", status: "verified" }), makeDomain({ id: "b", domain: "z.example.org" })] },
    { site: makeSite({ slug: "other" }), domains: [makeDomain({ id: "c", domain: "a.example.org", status: "verified" })] },
  ]);

  it("puts domains waiting for DNS first, then sorts by name", () => {
    expect(rows.map((row) => row.domain.id)).toEqual(["b", "c", "a"]);
  });

  it("counts the ones waiting", () => {
    expect(countWaiting(rows)).toBe(1);
  });

  it("returns nothing for sites without domains", () => {
    expect(toRows([{ site, domains: [] }])).toEqual([]);
  });
});
