import { Globe } from "lucide-react";
import type { JSX } from "react";
import { Link } from "react-router";
import { Alert } from "@/components/ui/alert";
import { EmptyState } from "@/components/ui/empty-state";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import { domainSlotsLeft } from "@/lib/domains";
import { canWrite } from "@/lib/roles";
import { useInstance } from "@/queries/instance";
import { useDomains } from "@/queries/site-detail";
import { AddDomainForm } from "./add-domain-form";
import { DomainCard } from "./domain-card";
import { useSiteContext } from "./site-context";

function ServerAddresses({ url, subdomain, path }: { url: string; subdomain: boolean; path: boolean }): JSX.Element {
  return (
    <Section title="Built-in addresses" description="The server gives every site these addresses without any DNS work.">
      <ul className="flex flex-col gap-1 text-sm">
        <li>
          Subdomain address: <strong className="font-medium">{subdomain ? "on" : "off"}</strong>
        </li>
        <li>
          Path address: <strong className="font-medium">{path ? "on" : "off"}</strong>
        </li>
        <li>
          Current address: <span className="break-all font-mono text-[0.8125rem]">{url}</span>
        </li>
      </ul>
    </Section>
  );
}

export function DomainsTab(): JSX.Element {
  const { site, isRoot } = useSiteContext();
  const domains = useDomains(site.slug);
  const instance = useInstance();
  const editable = canWrite(site.role);
  const modes = instance.data?.hostingModes;
  const limit = instance.data?.maxDomainsPerSite;
  const count = domains.data?.length ?? 0;
  const full = limit !== undefined && domainSlotsLeft(count, limit) === 0;

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      {modes && !modes.customDomains ? (
        <Alert tone="warning">
          <p>Custom domains are switched off for this server. Domains you add stay saved, but they serve nothing until an administrator turns the feature on.</p>
          {isRoot ? (
            <Link to="/admin/hosting" className="mt-1 inline-block font-medium underline underline-offset-4">
              Open hosting settings
            </Link>
          ) : null}
        </Alert>
      ) : null}
      {modes ? <ServerAddresses url={site.url ?? "No address available"} subdomain={modes.subdomain} path={modes.path} /> : null}
      <Section
        title="Custom domains"
        description={
          limit === undefined
            ? "Serve this site at your own domain names."
            : `Serve this site at your own domain names. ${count} of ${limit} used.`
        }
      >
        {editable ? (
          <>
            <AddDomainForm slug={site.slug} disabled={full} />
            {full ? <p className="text-sm text-muted">This site holds the most domains the server allows. Delete one to add another.</p> : null}
          </>
        ) : (
          <p className="text-sm text-muted">You have read-only access to this site, so you cannot add or change domains.</p>
        )}
        {domains.isPending ? <BlockSkeleton label="Loading domains" rows={2} /> : null}
        {domains.isError ? <LoadError error={domains.error} onRetry={() => void domains.refetch()} /> : null}
        {domains.data && domains.data.length === 0 ? (
          <EmptyState icon={<Globe />} title="No custom domains">
            <p>Add a domain, publish the TXT record and check it. Then point the domain at this server.</p>
          </EmptyState>
        ) : null}
        {domains.data && domains.data.length > 0 ? (
          <ul aria-label="Custom domains" className="-mx-4 divide-y divide-line border-y border-line sm:-mx-5">
            {domains.data.map((domain) => (
              <DomainCard key={domain.id} slug={site.slug} domain={domain} editable={editable} />
            ))}
          </ul>
        ) : null}
      </Section>
    </div>
  );
}
