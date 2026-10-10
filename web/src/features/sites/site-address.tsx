import { Copy, ExternalLink } from "lucide-react";
import type { JSX } from "react";
import { Button } from "@/components/ui/button";
import { useToast } from "@/components/ui/toast";
import { copyText } from "@/lib/clipboard";
import { displayAddress } from "@/lib/format";

export function SiteAddress({ url, name }: { url: string | null; name: string }): JSX.Element {
  const toast = useToast();

  if (url === null) {
    return <span className="text-sm text-muted">No address available</span>;
  }

  async function copy(): Promise<void> {
    if (url !== null && (await copyText(url))) {
      toast.success("Address copied.");
    } else {
      toast.error("Could not copy the address.");
    }
  }

  return (
    <div className="flex min-w-0 items-center gap-1">
      <span className="min-w-0 truncate rounded-md border border-line bg-sunken px-2 py-1 font-mono text-[0.8125rem]" title={url}>
        {displayAddress(url)}
      </span>
      <Button variant="ghost" size="icon" className="size-8" aria-label={`Copy address of ${name}`} onClick={() => void copy()}>
        <Copy />
      </Button>
      <Button asChild variant="ghost" size="icon" className="size-8">
        <a href={url} target="_blank" rel="noopener noreferrer" aria-label={`Open ${name} in a new tab`}>
          <ExternalLink />
        </a>
      </Button>
    </div>
  );
}
