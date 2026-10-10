import { describe, expect, it } from "vitest";
import {
  defaultInterval,
  hourlyAllowed,
  initialSelection,
  rangeDays,
  rangeForPreset,
  readSelection,
  selectCustom,
  selectPreset,
  toQuery,
  validateRange,
  writeSelection,
} from "./analytics-range";

const now = new Date("2026-03-10T15:30:00Z");

describe("rangeForPreset", () => {
  it("counts today as one day and the others back from today", () => {
    expect(rangeForPreset("today", now)).toEqual({ from: "2026-03-10", to: "2026-03-10" });
    expect(rangeForPreset("7d", now)).toEqual({ from: "2026-03-04", to: "2026-03-10" });
    expect(rangeForPreset("30d", now)).toEqual({ from: "2026-02-09", to: "2026-03-10" });
    expect(rangeForPreset("90d", now)).toEqual({ from: "2025-12-11", to: "2026-03-10" });
  });

  it("uses the UTC date near midnight", () => {
    expect(rangeForPreset("today", new Date("2026-03-10T23:59:59Z")).to).toBe("2026-03-10");
    expect(rangeForPreset("today", new Date("2026-03-11T00:00:00Z")).to).toBe("2026-03-11");
  });
});

describe("validateRange", () => {
  it("accepts a normal range", () => {
    expect(validateRange("2026-03-01", "2026-03-10")).toBeNull();
  });

  it("rejects missing, reversed and oversized ranges", () => {
    expect(validateRange("", "2026-03-10")).toBe("Enter both dates.");
    expect(validateRange("2026-02-30", "2026-03-10")).toBe("Enter both dates.");
    expect(validateRange("2026-03-10", "2026-03-01")).toBe("The start date must not be after the end date.");
    expect(validateRange("2025-01-01", "2026-03-10")).toBe("Pick a range of 366 days or fewer.");
  });
});

describe("intervals", () => {
  it("allows hours only up to 14 days", () => {
    expect(hourlyAllowed("2026-03-01", "2026-03-14")).toBe(true);
    expect(hourlyAllowed("2026-03-01", "2026-03-15")).toBe(false);
    expect(rangeDays("2026-03-01", "2026-03-14")).toBe(14);
  });

  it("defaults today to hours and everything else to days", () => {
    expect(defaultInterval(1, "today")).toBe("hour");
    expect(defaultInterval(7, "7d")).toBe("day");
    expect(selectPreset("today", now).interval).toBe("hour");
  });

  it("falls back to days when a custom range is too long for hours", () => {
    expect(selectCustom("2026-01-01", "2026-03-01", "hour").interval).toBe("day");
    expect(selectCustom("2026-03-01", "2026-03-05", "hour").interval).toBe("hour");
  });
});

describe("URL state", () => {
  it("round-trips a preset", () => {
    const selection = selectPreset("7d", now);
    const search = writeSelection(new URLSearchParams("tab=x"), selection);
    expect(search.toString()).toBe("tab=x&range=7d&interval=day");
    expect(readSelection(search, now)).toEqual(selection);
  });

  it("round-trips a custom range", () => {
    const selection = selectCustom("2026-03-01", "2026-03-05", "hour");
    const search = writeSelection(new URLSearchParams(), selection);
    expect(readSelection(search, now)).toEqual(selection);
  });

  it("falls back to 30 days for junk", () => {
    expect(readSelection(new URLSearchParams("range=nope"), now)).toEqual(initialSelection(now));
    expect(readSelection(new URLSearchParams("range=custom&from=bad&to=worse"), now)).toEqual(initialSelection(now));
  });

  it("ignores an hourly interval on a long preset", () => {
    expect(readSelection(new URLSearchParams("range=30d&interval=hour"), now).interval).toBe("day");
  });

  it("builds the API query", () => {
    expect(toQuery(selectPreset("7d", now))).toEqual({ from: "2026-03-04", to: "2026-03-10", interval: "day", top: 10 });
  });
});
