import { lazy, Suspense, type JSX } from "react";
import { Route, Routes } from "react-router";
import { InvitePage } from "@/features/auth/invite-page";
import { LoginPage } from "@/features/auth/login-page";
import { ResetConfirmPage } from "@/features/auth/reset-confirm-page";
import { ResetRequestPage } from "@/features/auth/reset-request-page";
import { SetupPage } from "@/features/auth/setup-page";
import { AccessTab } from "@/features/site-detail/access-tab";
import { DomainsTab } from "@/features/site-detail/domains-tab";
import { VersionsTab } from "@/features/site-detail/versions-tab";
import { OverviewTab } from "@/features/site-detail/overview-tab";
import { SiteLayout } from "@/features/site-detail/site-layout";
import { SettingsPage } from "@/features/settings/settings-page";
import { SitesPage } from "@/features/sites/sites-page";
import { AppShell } from "@/layout/app-shell";
import { NotFoundPage } from "@/layout/not-found-page";
import { RequireAuth } from "@/layout/require-auth";
import { Spinner } from "@/components/ui/spinner";

const AnalyticsTab = lazy(() => import("@/features/site-detail/analytics-tab").then((module) => ({ default: module.AnalyticsTab })));
const DocsPage = lazy(() => import("@/features/docs/docs-page").then((module) => ({ default: module.DocsPage })));

export function App(): JSX.Element {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/invite/:token" element={<InvitePage />} />
      <Route path="/reset-password" element={<ResetRequestPage />} />
      <Route path="/reset-password/:token" element={<ResetConfirmPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<AppShell />}>
          <Route index element={<SitesPage />} />
          <Route path="workspaces/:workspace" element={<SitesPage />} />
          <Route path="sites/:slug" element={<SiteLayout />}>
            <Route index element={<OverviewTab />} />
            <Route path="access" element={<AccessTab />} />
            <Route path="domains" element={<DomainsTab />} />
            <Route path="versions" element={<VersionsTab />} />
            <Route path="analytics" element={<Suspense fallback={<Spinner label="Loading analytics…" />}><AnalyticsTab /></Suspense>} />
          </Route>
          <Route
            path="docs/*"
            element={
              <Suspense fallback={<Spinner label="Loading docs…" />}>
                <DocsPage />
              </Suspense>
            }
          />
          <Route path="settings/*" element={<SettingsPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  );
}
