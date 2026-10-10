import type { JSX, ReactNode } from "react";
import { cn } from "@/lib/cn";

export interface Column<T> {
  key: string;
  header: string;
  cell: (row: T) => ReactNode;
  className?: string;
  hideLabelOnPhone?: boolean;
}

interface DataTableProps<T> {
  caption: string;
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
}

export function DataTable<T>({ caption, columns, rows, rowKey }: DataTableProps<T>): JSX.Element {
  return (
    <div className="overflow-hidden rounded-lg border border-line bg-surface">
      <table className="w-full border-collapse text-sm max-md:block">
        <caption className="sr-only">{caption}</caption>
        <thead className="border-b border-line bg-sunken text-left text-xs uppercase tracking-wider text-muted max-md:sr-only">
          <tr>
            {columns.map((column) => (
              <th key={column.key} scope="col" className={cn("px-4 py-2.5 font-medium", column.className)}>
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="max-md:block">
          {rows.map((row) => (
            <tr key={rowKey(row)} className="border-b border-line last:border-b-0 max-md:block max-md:px-4 max-md:py-3">
              {columns.map((column) => (
                <td
                  key={column.key}
                  data-label={column.hideLabelOnPhone ? undefined : column.header}
                  className={cn(
                    "px-4 py-3 align-middle max-md:flex max-md:items-baseline max-md:justify-between max-md:gap-4 max-md:px-0 max-md:py-1",
                    !column.hideLabelOnPhone && "max-md:before:shrink-0 max-md:before:text-xs max-md:before:text-muted max-md:before:content-[attr(data-label)]",
                    column.className,
                  )}
                >
                  <span className="min-w-0 max-md:text-right">{column.cell(row)}</span>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
