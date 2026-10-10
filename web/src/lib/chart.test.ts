import { describe, expect, it } from "vitest";
import {
  areaPath,
  buildPoints,
  formatAxisNumber,
  indexAt,
  linePath,
  roundedTopBar,
  maxValue,
  niceCeiling,
  pointLabel,
  summarise,
  tickIndices,
  xCentre,
  yFor,
  yTicks,
  type Layout,
} from "./chart";

const layout: Layout = { width: 400, height: 200, margins: { top: 10, right: 10, bottom: 30, left: 40 } };

describe("niceCeiling and ticks", () => {
  it("rounds up to a readable ceiling", () => {
    expect(niceCeiling(0)).toBe(4);
    expect(niceCeiling(3)).toBe(4);
    expect(niceCeiling(7)).toBe(10);
    expect(niceCeiling(11)).toBe(20);
    expect(niceCeiling(37)).toBe(40);
    expect(niceCeiling(120)).toBe(200);
    expect(niceCeiling(4200)).toBe(5000);
  });

  it("builds evenly spaced ticks from zero", () => {
    expect(yTicks(40)).toEqual([0, 10, 20, 30, 40]);
  });
});

describe("points", () => {
  const report = {
    interval: "day" as const,
    series: [
      { start: "2026-03-01T00:00:00Z", views: 5, uniques: 3 },
      { start: "2026-03-02T00:00:00Z", views: 12, uniques: 7 },
    ],
  };

  it("builds labelled points", () => {
    const points = buildPoints(report);
    expect(points).toHaveLength(2);
    expect(points[0]).toMatchObject({ index: 0, views: 5, uniques: 3, label: "1 Mar" });
    expect(points[1]?.fullLabel).toBe("Mon, 2 Mar 2026");
    expect(maxValue(points)).toBe(12);
  });

  it("labels hours in UTC", () => {
    expect(pointLabel("2026-03-01T14:00:00Z", "hour")).toEqual({ label: "14:00", fullLabel: "1 Mar, 14:00 UTC" });
  });

  it("summarises for assistive technology", () => {
    expect(summarise(buildPoints(report))).toBe("2 points from Sun, 1 Mar 2026 to Mon, 2 Mar 2026. 17 views in total. Busiest: Mon, 2 Mar 2026 with 12.");
    expect(summarise([])).toBe("No data.");
  });
});

describe("scales", () => {
  it("maps values to the plot area", () => {
    expect(yFor(0, 40, layout)).toBe(170);
    expect(yFor(40, 40, layout)).toBe(10);
    expect(yFor(20, 40, layout)).toBe(90);
    expect(yFor(-5, 40, layout)).toBe(170);
  });

  it("centres points in equal slots", () => {
    expect(xCentre(0, 2, layout)).toBe(40 + 175 / 2);
    expect(xCentre(1, 2, layout)).toBe(40 + 175 + 175 / 2);
  });

  it("finds the nearest slot for a pointer position", () => {
    expect(indexAt(0, 4, layout)).toBe(0);
    expect(indexAt(200, 4, layout)).toBe(1);
    expect(indexAt(999, 4, layout)).toBe(3);
    expect(indexAt(50, 0, layout)).toBe(0);
  });

  it("draws line and area paths", () => {
    expect(linePath([0, 40], 40, layout)).toBe("M127.5 170.0 L302.5 10.0");
    expect(areaPath([0, 40], 40, layout)).toBe("M127.5 170.0 L302.5 10.0 L302.5 170 L127.5 170 Z");
    expect(areaPath([], 40, layout)).toBe("");
  });
});

describe("tickIndices and number format", () => {
  it("keeps every index when they fit", () => {
    expect(tickIndices(3, 5)).toEqual([0, 1, 2]);
  });

  it("spreads ticks evenly and keeps both ends", () => {
    const ticks = tickIndices(30, 6);
    expect(ticks[0]).toBe(0);
    expect(ticks[ticks.length - 1]).toBe(29);
    expect(ticks.length).toBeLessThanOrEqual(6);
  });

  it("shortens large numbers", () => {
    expect(formatAxisNumber(950)).toBe("950");
    expect(formatAxisNumber(1500)).toBe("1.5k");
    expect(formatAxisNumber(12_000)).toBe("12k");
    expect(formatAxisNumber(2_500_000)).toBe("2.5M");
  });
});

describe("roundedTopBar", () => {
  it("is empty for a zero-height bar", () => {
    expect(roundedTopBar(10, 100, 8, 0, 4)).toBe("");
  });

  it("limits the radius to the bar size", () => {
    expect(roundedTopBar(10, 100, 4, 2, 4)).toBe("M10.0 102.0 V102.0 Q10.0 100.0 12.0 100.0 H12.0 Q14.0 100.0 14.0 102.0 V102.0 Z");
  });
});
