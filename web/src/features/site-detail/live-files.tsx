import { useState, type JSX } from "react";
import { Input, Label } from "@/components/ui/field";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import { formatBytes, pluralise } from "@/lib/format";
import { FILE_RENDER_LIMIT, filterFiles } from "@/lib/versions";
import { useLiveFiles } from "@/queries/site-detail";

export function LiveFiles({ slug }: { slug: string }): JSX.Element {
  const files = useLiveFiles(slug);
  const [query, setQuery] = useState("");
  const all = files.data?.files ?? [];
  const matches = filterFiles(all, query);
  const shown = matches.slice(0, FILE_RENDER_LIMIT);
  return (
    <Section title="Files in the live version" description="A read-only list. Your agent publishes new files through the API.">
      {files.isPending ? <BlockSkeleton label="Loading files" rows={2} /> : null}
      {files.isError ? <LoadError error={files.error} onRetry={() => void files.refetch()} /> : null}
      {files.data && all.length === 0 ? <p className="text-sm text-muted">No files are live. Nothing has been published to this site yet.</p> : null}
      {all.length > 0 ? (
        <>
          <div className="flex flex-col gap-1.5 sm:w-80">
            <Label htmlFor="file-filter">Filter files</Label>
            <Input id="file-filter" type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="index.html" autoComplete="off" />
          </div>
          <p aria-live="polite" className="text-xs text-muted">
            {matches.length > shown.length ? `Showing the first ${shown.length} of ${matches.length} files.` : pluralise(matches.length, "file")}
          </p>
          <ul aria-label="Live files" className="max-h-96 divide-y divide-line overflow-auto rounded-md border border-line text-sm">
            {shown.map((file) => (
              <li key={file.path} className="flex items-baseline justify-between gap-4 px-3 py-1.5">
                <span className="min-w-0 break-all font-mono text-[0.8125rem]">{file.path}</span>
                <span className="shrink-0 text-xs text-muted">{formatBytes(file.size)}</span>
              </li>
            ))}
          </ul>
        </>
      ) : null}
    </Section>
  );
}
