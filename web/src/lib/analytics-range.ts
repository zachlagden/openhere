import type { AnalyticsQuery } from "@/api/types";

export type RangePreset = "today" | "7d" | "30d" | "90d" | "custom";
export type Interval = "day" | "hour";

export const MAX_RANGE_DAYS = 366;
export const MAX_HOURLY_DAYS = 14;

export const PRESETS: { id: Exclude<RangePreset, "custom">; label: string; days: number }[] = [
  { id: "today", label: "Today", days: 1 },
  { id: "7d", label: "7 days", days: 7 },
  { id: "30d", label: "30 days", days: 30 },
  { id: "90d", label: "90 days", days: 90 },
];

export interface Selection {
  preset: RangePreset;
  from: string;
  to: string;
  interval: Interval;
}

const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/;
const DAY_MS = 86_400_000;

export function toIsoDay(date: Date): string {
  return date.toISOString().slice(0, 10);
}

export function parseIsoDay(value: string): Date | null {
  if (!DATE_PATTERN.test(value)) {
    return null;
  }
  const date = new Date(`${value}T00:00:00Z`);
  return Number.isNaN(date.getTime()) || toIsoDay(date) !== value ? null : date;
}

export function rangeDays(from: string, to: string): number {
  const start = parseIsoDay(from);
  const end = parseIsoDay(to);
  if (!start || !end) {
    return 0;
  }
  return Math.round((end.getTime() - start.getTime()) / DAY_MS) + 1;
}

export function rangeForPreset(preset: Exclude<RangePreset, "custom">, now: Date): { from: string; to: string } {
  const days = PRESETS.find((item) => item.id === preset)?.days ?? 30;
  const to = toIsoDay(now);
  const from = toIsoDay(new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() - (days - 1))));
  return { from, to };
}

export function defaultInterval(days: number, preset: RangePreset): Interval {
  return preset === "today" && days <= MAX_HOURLY_DAYS ? "hour" : "day";
}

export function validateRange(from: string, to: string): string | null {
  const start = parseIsoDay(from);
  const end = parseIsoDay(to);
  if (!start || !end) {
    return "Enter both dates.";
  }
  if (start.getTime() > end.getTime()) {
    return "The start date must not be after the end date.";
  }
  if (rangeDays(from, to) > MAX_RANGE_DAYS) {
    return `Pick a range of ${MAX_RANGE_DAYS} days or fewer.`;
  }
  return null;
}

export function hourlyAllowed(from: string, to: string): boolean {
  const days = rangeDays(from, to);
  return days >= 1 && days <= MAX_HOURLY_DAYS;
}

export function initialSelection(now: Date): Selection {
  const { from, to } = rangeForPreset("30d", now);
  return { preset: "30d", from, to, interval: "day" };
}

export function selectPreset(preset: Exclude<RangePreset, "custom">, now: Date): Selection {
  const { from, to } = rangeForPreset(preset, now);
  return { preset, from, to, interval: defaultInterval(rangeDays(from, to), preset) };
}

export function selectCustom(from: string, to: string, interval: Interval): Selection {
  const safeInterval = interval === "hour" && !hourlyAllowed(from, to) ? "day" : interval;
  return { preset: "custom", from, to, interval: safeInterval };
}

export function readSelection(search: URLSearchParams, now: Date): Selection {
  const range = search.get("range");
  const interval = search.get("interval") === "hour" ? "hour" : "day";
  if (range === "custom") {
    const from = search.get("from") ?? "";
    const to = search.get("to") ?? "";
    if (validateRange(from, to) === null) {
      return selectCustom(from, to, interval);
    }
    return initialSelection(now);
  }
  const preset = PRESETS.find((item) => item.id === range);
  if (!preset) {
    return initialSelection(now);
  }
  const selected = selectPreset(preset.id, now);
  const wantsHour = search.has("interval") ? interval === "hour" : selected.interval === "hour";
  return { ...selected, interval: wantsHour && hourlyAllowed(selected.from, selected.to) ? "hour" : "day" };
}

export function writeSelection(current: URLSearchParams, selection: Selection): URLSearchParams {
  const next = new URLSearchParams(current);
  next.set("range", selection.preset);
  next.set("interval", selection.interval);
  if (selection.preset === "custom") {
    next.set("from", selection.from);
    next.set("to", selection.to);
  } else {
    next.delete("from");
    next.delete("to");
  }
  return next;
}

export function toQuery(selection: Selection, top = 10): AnalyticsQuery {
  return { from: selection.from, to: selection.to, interval: selection.interval, top };
}
