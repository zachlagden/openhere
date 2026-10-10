import type { JSX } from "react";
import { CopyButton } from "@/components/ui/copy-field";
import type { DnsRecordRow } from "@/lib/domains";

function CopyCell({ label, value, mono = true }: { label: string; value: string; mono?: boolean }): JSX.Element {
  return (
    <div className="flex min-w-0 items-center gap-1">
      <span className={mono ? "min-w-0 break-all font-mono text-[0.8125rem]" : "min-w-0 break-all"}>{value}</span>
      <CopyButton value={value} label={`Copy ${label}`} done={`${label} copied.`} />
    </div>
  );
}

export function DnsRecords({ rows }: { rows: DnsRecordRow[] }): JSX.Element {
  return (
    <div className="flex flex-col gap-3">
      {rows.map((row) => (
        <div key={`${row.type}-${row.name}`} className="rounded-md border border-line bg-sunken px-3 py-2.5">
          <p className="text-xs font-medium uppercase tracking-wider text-muted">{row.purpose}</p>
          <dl className="mt-1.5 grid gap-x-4 gap-y-1 text-sm sm:grid-cols-[5rem_minmax(0,1fr)]">
            <dt className="text-muted">Type</dt>
            <dd className="font-mono text-[0.8125rem]">{row.type}</dd>
            <dt className="text-muted">Name</dt>
            <dd>
              <CopyCell label={`${row.type} record name`} value={row.name} />
            </dd>
            <dt className="text-muted">Value</dt>
            <dd>
              <CopyCell label={`${row.type} record value`} value={row.value} />
            </dd>
          </dl>
        </div>
      ))}
    </div>
  );
}
