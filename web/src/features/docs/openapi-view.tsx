import { Search } from "lucide-react";
import { useState, type JSX } from "react";
import { describeError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import { useApiReference } from "@/queries/api-reference";
import { parseInline } from "./markdown";
import { InlineView, MarkdownView } from "./markdown-view";
import { filterReference, type ApiReference, type Operation, type SchemaProperty } from "./openapi-model";

const METHOD_TONES: Record<string, string> = {
  GET: "bg-ok-soft text-ok",
  POST: "bg-accent-soft text-foreground",
  PUT: "bg-warn-soft text-warn",
  PATCH: "bg-warn-soft text-warn",
  DELETE: "bg-danger-soft text-danger",
};

function MethodTag({ method }: { method: string }): JSX.Element {
  return <span className={cn("w-16 shrink-0 rounded px-1.5 py-0.5 text-center font-mono text-xs font-semibold", METHOD_TONES[method] ?? "bg-sunken")}>{method}</span>;
}

function PropertyTable({ properties }: { properties: SchemaProperty[] }): JSX.Element {
  if (properties.length === 0) {
    return <p className="text-sm text-muted">No fields.</p>;
  }
  return (
    <div className="overflow-x-auto rounded-md border border-line">
      <table className="w-full min-w-[28rem] text-left text-sm">
        <thead className="bg-sunken text-xs uppercase tracking-wider text-muted">
          <tr>
            <th scope="col" className="px-3 py-1.5 font-medium">Field</th>
            <th scope="col" className="px-3 py-1.5 font-medium">Type</th>
            <th scope="col" className="px-3 py-1.5 font-medium">Notes</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {properties.map((property) => (
            <tr key={property.name} className="align-top">
              <td className="px-3 py-1.5 font-mono text-[0.8125rem]">
                {property.name}
                {property.required ? <span className="ml-1 text-xs text-danger">required</span> : null}
              </td>
              <td className="px-3 py-1.5 font-mono text-[0.8125rem] text-muted">{property.type}</td>
              <td className="px-3 py-1.5">
                <InlineView nodes={parseInline(property.description)} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function OperationDetails({ operation, reference }: { operation: Operation; reference: ApiReference }): JSX.Element {
  const body = operation.requestSchema ? reference.schemas[operation.requestSchema] : undefined;
  return (
    <div className="flex flex-col gap-4 border-t border-line px-4 py-4">
      {operation.description ? <MarkdownView source={operation.description} /> : null}
      {operation.parameters.length > 0 ? (
        <section aria-label="Parameters">
          <h4 className="mb-1.5 text-sm font-semibold">Parameters</h4>
          <ul className="space-y-1 text-sm">
            {operation.parameters.map((parameter) => (
              <li key={`${parameter.location}-${parameter.name}`}>
                <code className="font-mono text-[0.8125rem]">{parameter.name}</code>{" "}
                <span className="text-muted">in {parameter.location}{parameter.required ? ", required" : ""}.</span>{" "}
                <InlineView nodes={parseInline(parameter.description)} />
              </li>
            ))}
          </ul>
        </section>
      ) : null}
      {operation.requestSchema ? (
        <section aria-label="Request body">
          <h4 className="mb-1.5 text-sm font-semibold">
            Request body <span className="font-mono text-xs font-normal text-muted">{operation.requestSchema}</span>
          </h4>
          {body ? <PropertyTable properties={body} /> : null}
        </section>
      ) : null}
      <section aria-label="Responses">
        <h4 className="mb-1.5 text-sm font-semibold">Responses</h4>
        <ul className="space-y-1 text-sm">
          {operation.responses.map((response) => (
            <li key={response.status} className="flex gap-2">
              <span className="w-10 shrink-0 font-mono text-[0.8125rem]">{response.status}</span>
              <span className="min-w-0 text-muted">{response.description.split("\n")[0]}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

function OperationView({ operation, reference }: { operation: Operation; reference: ApiReference }): JSX.Element {
  return (
    <details className="group rounded-lg border border-line bg-surface">
      <summary className="flex cursor-pointer list-none flex-wrap items-center gap-x-3 gap-y-1 rounded-lg px-4 py-2.5 hover:bg-sunken">
        <MethodTag method={operation.method} />
        <code className="min-w-0 break-all font-mono text-[0.8125rem]">{operation.path}</code>
        <span className="min-w-0 text-sm text-muted">{operation.summary}</span>
        {operation.public ? <Badge className="ml-auto">No sign-in needed</Badge> : null}
      </summary>
      <OperationDetails operation={operation} reference={reference} />
    </details>
  );
}

export function OpenApiView(): JSX.Element {
  const reference = useApiReference();
  const [query, setQuery] = useState("");

  if (reference.isPending) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-10 w-full max-w-md" />
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
      </div>
    );
  }
  if (reference.isError) {
    return <Alert tone="error">{describeError(reference.error).message}</Alert>;
  }

  const groups = filterReference(reference.data, query);
  return (
    <div className="flex flex-col gap-6">
      <div className="max-w-3xl">
        <p className="text-sm text-muted">
          {reference.data.title}, version {reference.data.version}. Paths start with <code className="font-mono text-xs">/api/v1</code>. The raw
          document is at <code className="font-mono text-xs">/api/v1/docs/openapi.yaml</code>.
        </p>
      </div>
      <div className="relative max-w-md">
        <Search aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" />
        <Input
          type="search"
          aria-label="Filter endpoints"
          placeholder="Filter endpoints"
          className="pl-9"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </div>
      {groups.length === 0 ? <p className="text-sm text-muted">No endpoint matches that filter.</p> : null}
      {groups.map((group) => (
        <section key={group.name} aria-labelledby={`tag-${group.name}`} className="flex flex-col gap-2">
          <h2 id={`tag-${group.name}`} className="font-display text-xl font-medium">
            {group.name}
          </h2>
          {group.operations.map((operation) => (
            <OperationView key={operation.id} operation={operation} reference={reference.data} />
          ))}
        </section>
      ))}
    </div>
  );
}
