import type { JSX } from "react";
import type { ChartPoint } from "@/lib/chart";
import { formatCount } from "@/lib/format";

export function ChartTable({ points }: { points: ChartPoint[] }): JSX.Element {
  return (
    <div className="max-h-80 overflow-auto rounded-md border border-line">
      <table className="w-full border-collapse text-sm">
        <caption className="sr-only">Views and unique visitors for each point in the chart</caption>
        <thead className="sticky top-0 bg-sunken text-left text-xs uppercase tracking-wider text-muted">
          <tr>
            <th scope="col" className="px-3 py-2 font-medium">
              Time
            </th>
            <th scope="col" className="px-3 py-2 text-right font-medium">
              Views
            </th>
            <th scope="col" className="px-3 py-2 text-right font-medium">
              Unique visitors
            </th>
          </tr>
        </thead>
        <tbody>
          {points.map((point) => (
            <tr key={point.start} className="border-t border-line">
              <th scope="row" className="px-3 py-1.5 text-left font-normal">
                {point.fullLabel}
              </th>
              <td className="px-3 py-1.5 text-right tabular-nums">{formatCount(point.views)}</td>
              <td className="px-3 py-1.5 text-right tabular-nums">{formatCount(point.uniques)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
