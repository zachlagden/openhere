import type { JSX } from "react";
import type { Analytics } from "@/api/types";
import { formatCount } from "@/lib/format";

function Tile({ label, value, note }: { label: string; value: number; note: string }): JSX.Element {
  return (
    <div className="rounded-lg border border-line bg-surface px-4 py-3.5">
      <dt className="text-xs font-medium uppercase tracking-wider text-muted">{label}</dt>
      <dd className="mt-1 font-display text-3xl font-medium tabular-nums" data-testid={`total-${label.toLowerCase().replace(/\s+/g, "-")}`}>
        {formatCount(value)}
      </dd>
      <p className="mt-1 text-xs text-muted">{note}</p>
    </div>
  );
}

export function Totals({ totals }: { totals: Analytics["totals"] }): JSX.Element {
  return (
    <dl className="grid gap-3 sm:grid-cols-3">
      <Tile label="Views" value={totals.views} note="HTML pages served to people." />
      <Tile label="Unique visitors" value={totals.uniques} note="Counted once per site per day, so a visitor who returns on three days counts three times." />
      <Tile label="Bot views" value={totals.botViews} note="Counted here only. Left out of every other number." />
    </dl>
  );
}
