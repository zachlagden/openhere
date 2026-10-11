import { ExternalLink } from "lucide-react";
import type { JSX } from "react";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { displayAddress } from "@/lib/format";

export function SiteAddress({ url, name }: { url: string | null; name: string }): JSX.Element {
  if (url === null) {
    return <span className="text-sm text-muted">No address available</span>;
  }

  return (
    <div className="flex min-w-0 items-center gap-1">
      <span className="min-w-0 truncate rounded-md border border-line bg-sunken px-2 py-1 font-mono text-[0.8125rem]" title={url}>
        {displayAddress(url)}
      </span>
      <CopyButton value={url} label={`Copy address of ${name}`} />
      <Button asChild variant="ghost" size="icon" className="size-8">
        <a href={url} target="_blank" rel="noopener noreferrer" aria-label={`Open ${name} in a new tab`}>
          <ExternalLink />
        </a>
      </Button>
    </div>
  );
}
