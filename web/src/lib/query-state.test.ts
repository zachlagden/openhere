import { describe, expect, it } from "vitest";
import { hasActiveFilters, isApiPath, loginPathFor, readSitesParams, safeNextPath, writeSitesParams } from "./query-state";

describe("sites params", () => {
  it("reads and trims", () => {
    expect(readSitesParams(new URLSearchParams("q=%20launch%20&folder=f1"))).toEqual({ q: "launch", folder: "f1" });
    expect(readSitesParams(new URLSearchParams(""))).toEqual({ q: "", folder: "" });
  });

  it("writes a patch and removes empty values", () => {
    const next = writeSitesParams(new URLSearchParams("q=a&folder=f1&other=1"), { q: "", folder: "none" });
    expect(next.toString()).toBe("folder=none&other=1");
  });

  it("detects active filters", () => {
    expect(hasActiveFilters({ q: "", folder: "" })).toBe(false);
    expect(hasActiveFilters({ q: "x", folder: "" })).toBe(true);
  });
});

describe("safeNextPath", () => {
  it("keeps local paths", () => {
    expect(safeNextPath("/workspaces/acme?folder=f1")).toBe("/workspaces/acme?folder=f1");
    expect(safeNextPath("/api/v1/access/handoff?site=a")).toBe("/api/v1/access/handoff?site=a");
  });

  it("rejects anything that could leave the site", () => {
    for (const bad of ["https://evil.example.com", "//evil.example.com", "/\\evil.example.com", "javascript:alert(1)", "", null, undefined, "/a\nb"]) {
      expect(safeNextPath(bad)).toBe("/");
    }
  });

  it("tells API paths apart", () => {
    expect(isApiPath("/api/v1/me")).toBe(true);
    expect(isApiPath("/apiary")).toBe(false);
  });

  it("builds the login path", () => {
    expect(loginPathFor("/")).toBe("/login");
    expect(loginPathFor("/docs?x=1")).toBe("/login?next=%2Fdocs%3Fx%3D1");
  });
});
