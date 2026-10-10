import { useState, type FormEvent, type JSX } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/field";
import {
  hourlyAllowed,
  MAX_HOURLY_DAYS,
  PRESETS,
  selectCustom,
  selectPreset,
  validateRange,
  type Interval,
  type Selection,
} from "@/lib/analytics-range";
import { cn } from "@/lib/cn";

interface RangeControlsProps {
  selection: Selection;
  onChange: (selection: Selection) => void;
}

export function RangeControls({ selection, onChange }: RangeControlsProps): JSX.Element {
  const [from, setFrom] = useState(selection.from);
  const [to, setTo] = useState(selection.to);
  const [error, setError] = useState<string | null>(null);
  const hourly = hourlyAllowed(selection.from, selection.to);

  function applyCustom(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const problem = validateRange(from, to);
    setError(problem);
    if (problem === null) {
      onChange(selectCustom(from, to, selection.interval));
    }
  }

  function changeInterval(value: string): void {
    const interval: Interval = value === "hour" ? "hour" : "day";
    onChange({ ...selection, interval });
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-end gap-x-6 gap-y-3">
        <div role="group" aria-label="Date range" className="inline-flex overflow-hidden rounded-md border border-line-strong">
          {PRESETS.map((preset) => (
            <button
              key={preset.id}
              type="button"
              aria-pressed={selection.preset === preset.id}
              onClick={() => {
                const next = selectPreset(preset.id, new Date());
                setFrom(next.from);
                setTo(next.to);
                setError(null);
                onChange(next);
              }}
              className={cn(
                "h-9 border-r border-line-strong px-3 text-sm last:border-r-0",
                selection.preset === preset.id ? "bg-accent-soft font-medium text-foreground" : "bg-surface text-muted hover:bg-sunken hover:text-foreground",
              )}
            >
              {preset.label}
            </button>
          ))}
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="analytics-interval">Chart interval</Label>
          <Select id="analytics-interval" className="w-32" value={selection.interval} onChange={(event) => changeInterval(event.target.value)}>
            <option value="day">By day</option>
            <option value="hour" disabled={!hourly}>
              By hour
            </option>
          </Select>
        </div>
      </div>
      <form onSubmit={applyCustom} className="flex flex-wrap items-end gap-3" aria-label="Custom range">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="analytics-from">From</Label>
          <Input id="analytics-from" type="date" className="w-40" value={from} onChange={(event) => setFrom(event.target.value)} required />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="analytics-to">To</Label>
          <Input id="analytics-to" type="date" className="w-40" value={to} onChange={(event) => setTo(event.target.value)} required />
        </div>
        <Button type="submit" variant="secondary">
          Apply range
        </Button>
        <p className="basis-full text-xs text-muted">
          Dates are in UTC. A range can span up to 366 days. Hourly charts need {MAX_HOURLY_DAYS} days or fewer.
        </p>
        {error ? (
          <p role="alert" className="basis-full text-xs text-danger">
            {error}
          </p>
        ) : null}
      </form>
    </div>
  );
}
