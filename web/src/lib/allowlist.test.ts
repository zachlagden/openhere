import { describe, expect, it } from "vitest";
import { addEntry, normaliseDomain, normaliseEmail, removeEntry } from "./allowlist";

describe("normaliseEmail", () => {
  it("lower-cases and trims", () => {
    expect(normaliseEmail("  Sam@Example.COM ")).toEqual({ ok: true, value: "sam@example.com" });
  });

  it.each(["", "sam", "sam@", "@example.com", "sam@example", "sam example@example.com", "sám@example.com", "sam@exam_ple.com"])("rejects %j", (raw) => {
    expect(normaliseEmail(raw).ok).toBe(false);
  });

  it("rejects an address longer than 254 characters", () => {
    expect(normaliseEmail(`a@${"b".repeat(60)}.${"c".repeat(60)}.${"d".repeat(60)}.${"e".repeat(60)}.${"f".repeat(30)}.com`).ok).toBe(false);
  });
});

describe("normaliseDomain", () => {
  it("accepts a leading at sign and lower-cases", () => {
    expect(normaliseDomain("@Example.com")).toEqual({ ok: true, value: "example.com" });
  });

  it.each(["", "localhost", "*.example.com", "https://example.com", "example.com/path", "exa mple.com", "-bad.example.com", "example..com"])("rejects %j", (raw) => {
    expect(normaliseDomain(raw).ok).toBe(false);
  });
});

describe("addEntry", () => {
  it("adds a normalised entry in sorted order", () => {
    expect(addEntry(["sam@example.com"], "email", "Alex@example.com")).toEqual({ ok: true, list: ["alex@example.com", "sam@example.com"] });
  });

  it("refuses a duplicate", () => {
    const result = addEntry(["example.com"], "domain", "@example.com");
    expect(result).toEqual({ ok: false, message: "example.com is already on the list." });
  });

  it("refuses an entry beyond the limit", () => {
    const full = Array.from({ length: 500 }, (_, index) => `site${index}.example.com`);
    expect(addEntry(full, "domain", "extra.example.com").ok).toBe(false);
  });

  it("reports a validation message for a bad entry", () => {
    expect(addEntry([], "email", "nope")).toEqual({ ok: false, message: "Enter a plain email address such as sam@example.com." });
  });
});

describe("removeEntry", () => {
  it("removes one entry", () => {
    expect(removeEntry(["a.example.com", "b.example.com"], "a.example.com")).toEqual(["b.example.com"]);
  });
});
