import { describe, expect, it } from "vitest";
import type { Version } from "@/api/types";
import { canDelete, canRestore, describeAuthor, filterFiles, versionLabel } from "./versions";

const base: Version = {
  id: "0123456789abcdef",
  siteId: "s1",
  status: "finalized",
  live: false,
  createdBy: "u1",
  createdAt: "2026-03-01T10:00:00Z",
  finalizedAt: "2026-03-01T10:01:00Z",
  fileCount: 2,
  totalBytes: 100,
  baseVersionId: null,
};

describe("version rules", () => {
  it("restores only finalized versions that are not live", () => {
    expect(canRestore(base)).toBe(true);
    expect(canRestore({ ...base, live: true })).toBe(false);
    expect(canRestore({ ...base, status: "pending", finalizedAt: null })).toBe(false);
  });

  it("deletes anything that is not live", () => {
    expect(canDelete(base)).toBe(true);
    expect(canDelete({ ...base, status: "pending", finalizedAt: null })).toBe(true);
    expect(canDelete({ ...base, live: true })).toBe(false);
  });

  it("shortens the id", () => {
    expect(versionLabel(base)).toBe("01234567");
    expect(versionLabel({ ...base, id: "v1" })).toBe("v1");
  });

  it("names the author without exposing other users", () => {
    expect(describeAuthor("u1", "u1")).toBe("You");
    expect(describeAuthor("u2", "u1")).toBe("Another member");
    expect(describeAuthor(null, "u1")).toBe("Unknown");
  });
});

describe("filterFiles", () => {
  const files = [
    { path: "index.html", size: 1, sha256: "a", contentType: "text/html" },
    { path: "assets/App.js", size: 2, sha256: "b", contentType: "text/javascript" },
  ];

  it("filters by path ignoring case", () => {
    expect(filterFiles(files, "app")).toEqual([files[1]]);
    expect(filterFiles(files, "  ")).toEqual(files);
    expect(filterFiles(files, "zzz")).toEqual([]);
  });
});
