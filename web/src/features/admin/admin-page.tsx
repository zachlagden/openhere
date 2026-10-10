import type { JSX } from "react";
import { Navigate, NavLink, Route, Routes } from "react-router";
import { PageHeader } from "@/components/ui/page-header";
import { NotFoundPage } from "@/layout/not-found-page";
import { cn } from "@/lib/cn";
import { usePageTitle } from "@/lib/use-page-title";
import { useMe } from "@/queries/session";
import { AnalyticsSection } from "./analytics-section";
import { EmailSection } from "./email-section";
import { HostingSection } from "./hosting-section";
import { PolicySection } from "./policy-section";
import { UsersSection } from "./users-section";

const SECTIONS = [
  { path: "hosting", label: "Hosting" },
  { path: "email", label: "Email" },
  { path: "users", label: "Users" },
  { path: "policy", label: "Access and policy" },
  { path: "analytics", label: "Analytics" },
] as const;

export function AdminPage(): JSX.Element {
  const me = useMe();
  usePageTitle("Instance admin");
  if (!me.data?.isRoot) {
    return <NotFoundPage />;
  }
  return (
    <>
      <PageHeader title="Instance admin" description="Settings for the whole server. Only the root user sees this area." />
      <div className="grid gap-6 md:grid-cols-[13rem_minmax(0,1fr)]">
        <nav aria-label="Admin sections" className="-mx-4 flex gap-1 overflow-x-auto px-4 md:mx-0 md:flex-col md:px-0">
          {SECTIONS.map((section) => (
            <NavLink
              key={section.path}
              to={`/admin/${section.path}`}
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
        <div className="min-w-0">
          <Routes>
            <Route index element={<Navigate to="hosting" replace />} />
            <Route path="hosting" element={<HostingSection />} />
            <Route path="email" element={<EmailSection />} />
            <Route path="users" element={<UsersSection />} />
            <Route path="policy" element={<PolicySection />} />
            <Route path="analytics" element={<AnalyticsSection />} />
            <Route path="*" element={<NotFoundPage />} />
          </Routes>
        </div>
      </div>
    </>
  );
}
