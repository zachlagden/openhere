import type { JSX } from "react";
import { NavLink, Route, Routes } from "react-router";
import { PageHeader } from "@/components/ui/page-header";
import { Spinner } from "@/components/ui/spinner";
import { NotFoundPage } from "@/layout/not-found-page";
import { cn } from "@/lib/cn";
import { usePageTitle } from "@/lib/use-page-title";
import { lazy, Suspense } from "react";
import { ApiKeysSection } from "./api-keys-section";
import { DomainsOverviewSection } from "./domains-overview-section";
import { GeneralSection } from "./general-section";
import { ProfileSection } from "./profile-section";

const PersonalAnalyticsSection = lazy(() => import("./personal-analytics-section").then((module) => ({ default: module.PersonalAnalyticsSection })));

const SECTIONS = [
  { path: "", label: "Profile" },
  { path: "general", label: "General" },
  { path: "api-keys", label: "API keys" },
  { path: "analytics", label: "Analytics" },
  { path: "domains", label: "Domains" },
] as const;

export function SettingsPage(): JSX.Element {
  usePageTitle("Settings");
  return (
    <>
      <PageHeader title="Settings" description="Your account and the sites you manage." />
      <div className="grid gap-6 md:grid-cols-[13rem_minmax(0,1fr)]">
        <nav aria-label="Settings sections" className="-mx-4 flex gap-1 overflow-x-auto px-4 md:mx-0 md:flex-col md:px-0">
          {SECTIONS.map((section) => (
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
        <div className="min-w-0">
          <Routes>
            <Route index element={<ProfileSection />} />
            <Route path="general" element={<GeneralSection />} />
            <Route path="api-keys" element={<ApiKeysSection />} />
            <Route
              path="analytics"
              element={
                <Suspense fallback={<Spinner label="Loading analytics…" />}>
                  <PersonalAnalyticsSection />
                </Suspense>
              }
            />
            <Route path="domains" element={<DomainsOverviewSection />} />
            <Route path="*" element={<NotFoundPage />} />
          </Routes>
        </div>
      </div>
    </>
  );
}
