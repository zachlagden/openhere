import { describe, expect, it } from "vitest";
import { displayAddress, formatRelativeTime, pluralise } from "./format";

const now = new Date("2026-03-10T12:00:00Z");

describe("formatRelativeTime", () => {
  it("describes recent and older times", () => {
    expect(formatRelativeTime("2026-03-10T11:59:50Z", now)).toBe("just now");
    expect(formatRelativeTime("2026-03-10T11:00:00Z", now)).toBe("1 hour ago");
    expect(formatRelativeTime("2026-03-08T12:00:00Z", now)).toBe("2 days ago");
    expect(formatRelativeTime("not a date", now)).toBe("");
  });
});

describe("helpers", () => {
  it("shortens addresses", () => {
    expect(displayAddress("https://sites.example.com/s/a/")).toBe("sites.example.com/s/a");
  });
  it("pluralises", () => {
    expect(pluralise(1, "site")).toBe("1 site");
    expect(pluralise(3, "site")).toBe("3 sites");
  });
});
