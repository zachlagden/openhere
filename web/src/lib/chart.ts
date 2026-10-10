import type { Analytics } from "@/api/types";

export type Interval = Analytics["interval"];

export interface ChartPoint {
  index: number;
  start: string;
  label: string;
  fullLabel: string;
  views: number;
  uniques: number;
}

export interface Margins {
  top: number;
  right: number;
  bottom: number;
  left: number;
}

export interface Layout {
  width: number;
  height: number;
  margins: Margins;
}

export function niceCeiling(value: number): number {
  if (!Number.isFinite(value) || value <= 4) {
    return 4;
  }
  const exponent = Math.floor(Math.log10(value));
  const base = 10 ** exponent;
  for (const step of [1, 2, 4, 5, 10]) {
    if (value <= step * base) {
      return step * base;
    }
  }
  return 10 * base;
}

export function yTicks(ceiling: number, count = 4): number[] {
  return Array.from({ length: count + 1 }, (_, index) => (ceiling / count) * index);
}

function formatter(options: Intl.DateTimeFormatOptions, locale: string): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat(locale, { ...options, timeZone: "UTC" });
}

export function pointLabel(start: string, interval: Interval, locale = "en-GB"): { label: string; fullLabel: string } {
  const date = new Date(start);
  if (Number.isNaN(date.getTime())) {
    return { label: "", fullLabel: "" };
  }
  if (interval === "hour") {
    return {
      label: formatter({ hour: "2-digit", minute: "2-digit", hourCycle: "h23" }, locale).format(date),
      fullLabel: `${formatter({ day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", hourCycle: "h23" }, locale).format(date)} UTC`,
    };
  }
  return {
    label: formatter({ day: "numeric", month: "short" }, locale).format(date),
    fullLabel: formatter({ weekday: "short", day: "numeric", month: "short", year: "numeric" }, locale).format(date),
  };
}

export function buildPoints(report: Pick<Analytics, "series" | "interval">, locale = "en-GB"): ChartPoint[] {
  return report.series.map((point, index) => ({
    index,
    start: point.start,
    views: point.views,
    uniques: point.uniques,
    ...pointLabel(point.start, report.interval, locale),
  }));
}

export function maxValue(points: ChartPoint[]): number {
  let max = 0;
  for (const point of points) {
    max = Math.max(max, point.views, point.uniques);
  }
  return max;
}

export function plotArea(layout: Layout): { x0: number; x1: number; y0: number; y1: number } {
  return {
    x0: layout.margins.left,
    x1: layout.width - layout.margins.right,
    y0: layout.margins.top,
    y1: layout.height - layout.margins.bottom,
  };
}

export function xCentre(index: number, count: number, layout: Layout): number {
  const { x0, x1 } = plotArea(layout);
  const slot = (x1 - x0) / Math.max(count, 1);
  return x0 + slot * index + slot / 2;
}

export function yFor(value: number, ceiling: number, layout: Layout): number {
  const { y0, y1 } = plotArea(layout);
  return y1 - (Math.max(0, value) / ceiling) * (y1 - y0);
}

export function indexAt(x: number, count: number, layout: Layout): number {
  const { x0, x1 } = plotArea(layout);
  if (count <= 0) {
    return 0;
  }
  const slot = (x1 - x0) / count;
  return Math.min(count - 1, Math.max(0, Math.floor((x - x0) / slot)));
}

export function linePath(values: number[], ceiling: number, layout: Layout): string {
  return values
    .map((value, index) => `${index === 0 ? "M" : "L"}${xCentre(index, values.length, layout).toFixed(1)} ${yFor(value, ceiling, layout).toFixed(1)}`)
    .join(" ");
}

export function areaPath(values: number[], ceiling: number, layout: Layout): string {
  if (values.length === 0) {
    return "";
  }
  const { y1 } = plotArea(layout);
  const first = xCentre(0, values.length, layout).toFixed(1);
  const last = xCentre(values.length - 1, values.length, layout).toFixed(1);
  return `${linePath(values, ceiling, layout)} L${last} ${y1} L${first} ${y1} Z`;
}

export function tickIndices(count: number, maxTicks: number): number[] {
  if (count <= 0) {
    return [];
  }
  if (count <= maxTicks) {
    return Array.from({ length: count }, (_, index) => index);
  }
  const step = (count - 1) / (maxTicks - 1);
  const picked = new Set<number>();
  for (let tick = 0; tick < maxTicks; tick += 1) {
    picked.add(Math.round(tick * step));
  }
  return [...picked].sort((left, right) => left - right);
}

export function maxTicksFor(width: number): number {
  return Math.max(2, Math.min(8, Math.floor(width / 80)));
}

export function formatAxisNumber(value: number): string {
  if (value >= 1_000_000) {
    return `${Number((value / 1_000_000).toFixed(1))}M`;
  }
  if (value >= 10_000) {
    return `${Number((value / 1000).toFixed(0))}k`;
  }
  if (value >= 1000) {
    return `${Number((value / 1000).toFixed(1))}k`;
  }
  return String(Math.round(value));
}

export function summarise(points: ChartPoint[]): string {
  if (points.length === 0) {
    return "No data.";
  }
  const views = points.reduce((total, point) => total + point.views, 0);
  const peak = points.reduce((best, point) => (point.views > best.views ? point : best), points[0]!);
  return `${points.length} points from ${points[0]!.fullLabel} to ${points[points.length - 1]!.fullLabel}. ${views} views in total. Busiest: ${peak.fullLabel} with ${peak.views}.`;
}

export function roundedTopBar(x: number, y: number, width: number, height: number, radius: number): string {
  if (height <= 0) {
    return "";
  }
  const r = Math.min(radius, width / 2, height);
  return `M${x.toFixed(1)} ${(y + height).toFixed(1)} V${(y + r).toFixed(1)} Q${x.toFixed(1)} ${y.toFixed(1)} ${(x + r).toFixed(1)} ${y.toFixed(1)} H${(x + width - r).toFixed(1)} Q${(x + width).toFixed(1)} ${y.toFixed(1)} ${(x + width).toFixed(1)} ${(y + r).toFixed(1)} V${(y + height).toFixed(1)} Z`;
}
