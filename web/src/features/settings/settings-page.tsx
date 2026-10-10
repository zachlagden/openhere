import type { JSX } from "react";
import { Cog } from "lucide-react";
import { Link, NavLink, Route, Routes } from "react-router";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { cn } from "@/lib/cn";
import { useMe } from "@/queries/session";

interface Section {
  path: string;
  label: string;
  rootOnly: boolean;
}

const SECTIONS: Section[] = [
  { path: "", label: "Profile", rootOnly: false },
  { path: "api-keys", label: "API keys", rootOnly: false },
  { path: "domains", label: "Domains", rootOnly: false },
  { path: "hosting", label: "Hosting", rootOnly: true },
];

function SectionPlaceholder({ label }: { label: string }): JSX.Element {
  return (
    <EmptyState
      icon={<Cog />}
      title={`${label} settings are not available in the dashboard yet`}
      actions={
        <Button asChild variant="secondary">
          <Link to="/docs/api">Open the API reference</Link>
        </Button>
      }
    >
      <p>The API already covers this. Your agent can manage it, or you can call the API directly.</p>
    </EmptyState>
  );
}

export function SettingsPage(): JSX.Element {
  const me = useMe();
  const isRoot = me.data?.isRoot ?? false;
  const sections = SECTIONS.filter((section) => isRoot || !section.rootOnly);
  return (
    <>
      <PageHeader title="Settings" />
      <div className="grid gap-6 md:grid-cols-[13rem_minmax(0,1fr)]">
        <nav aria-label="Settings sections" className="flex gap-1 overflow-x-auto md:flex-col">
          {sections.map((section) => (
            <NavLink
              key={section.path}
              to={section.path === "" ? "/settings" : `/settings/${section.path}`}
              end
              className={({ isActive }) =>
                cn(
                  "whitespace-nowrap rounded-md px-3 py-2 text-sm",
                  isActive ? "bg-accent-soft font-medium text-foreground" : "text-muted hover:bg-sunken hover:text-foreground",
                )
              }
            >
              {section.label}
            </NavLink>
          ))}
        </nav>
        <Routes>
          {sections.map((section) => (
            <Route
              key={section.path}
              path={section.path === "" ? undefined : section.path}
              index={section.path === ""}
              element={<SectionPlaceholder label={section.label} />}
            />
          ))}
        </Routes>
      </div>
    </>
  );
}
