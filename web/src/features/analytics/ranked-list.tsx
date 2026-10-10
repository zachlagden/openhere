import type { JSX, ReactNode } from "react";
import type { AnalyticsRanked } from "@/api/types";
import { barShares } from "@/lib/ranked";
import { formatCount } from "@/lib/format";

interface RankedListProps {
  title: string;
  items: AnalyticsRanked;
  note?: ReactNode;
  emptyText?: string;
}

export function RankedList({ title, items, note, emptyText = "Nothing recorded in this range." }: RankedListProps): JSX.Element {
  const shares = barShares(items.map((item) => item.views));
  return (
    <section aria-label={title} className="flex min-w-0 flex-col gap-2 rounded-lg border border-line bg-surface px-4 py-3.5">
      <h3 className="flex items-baseline justify-between gap-2 text-sm font-medium">
        {title}
        <span className="text-xs font-normal text-muted">Views</span>
      </h3>
      {note ? <div className="text-xs text-muted">{note}</div> : null}
      {items.length === 0 ? (
        <p className="text-sm text-muted">{emptyText}</p>
      ) : (
        <ol className="flex flex-col gap-1.5">
          {items.map((item, index) => (
            <li key={item.value} className="relative flex items-center justify-between gap-3 overflow-hidden rounded-sm px-2 py-1 text-sm">
              <span aria-hidden="true" className="absolute inset-y-0 left-0 rounded-sm bg-accent-soft" style={{ width: `${shares[index] ?? 0}%` }} />
              <span className="relative min-w-0 truncate" title={item.value}>
                {item.value}
              </span>
              <span className="relative shrink-0 tabular-nums" title={`${formatCount(item.uniques)} unique visitors`}>
                {formatCount(item.views)}
              </span>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
