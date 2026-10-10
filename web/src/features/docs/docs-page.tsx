import type { JSX } from "react";
import { NavLink, Route, Routes } from "react-router";
import { describeError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import { useAgentGuide } from "@/queries/instance";
import { MarkdownView } from "./markdown-view";
import { OpenApiView } from "./openapi-view";

function AgentGuide(): JSX.Element {
  const guide = useAgentGuide();
  if (guide.isPending) {
    return (
      <div className="flex max-w-3xl flex-col gap-3">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-5/6" />
      </div>
    );
  }
  if (guide.isError) {
    return <Alert tone="error">{describeError(guide.error).message}</Alert>;
  }
  return <MarkdownView source={guide.data} />;
}

function Tab({ to, end, children }: { to: string; end?: boolean; children: string }): JSX.Element {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cn(
          "-mb-px border-b-2 px-1 py-2 text-sm font-medium",
          isActive ? "border-accent text-foreground" : "border-transparent text-muted hover:text-foreground",
        )
      }
    >
      {children}
    </NavLink>
  );
}

export function DocsPage(): JSX.Element {
  return (
    <>
      <PageHeader title="Docs" description="How agents use this server, and the API they call." />
      <nav aria-label="Docs sections" className="mb-6 flex gap-5 border-b border-line">
        <Tab to="/docs" end>
          Agent guide
        </Tab>
        <Tab to="/docs/api">API reference</Tab>
      </nav>
      <Routes>
        <Route index element={<AgentGuide />} />
        <Route path="api" element={<OpenApiView />} />
      </Routes>
    </>
  );
}
