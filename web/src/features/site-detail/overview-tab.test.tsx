import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { alex, makeSite, personal, reportsFolder } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { OverviewTab } from "./overview-tab";
import { SiteLayout } from "./site-layout";

afterEach(() => {
  vi.unstubAllGlobals();
});

function routes(site = makeSite(), extra: Parameters<typeof mockApi>[0] = {}): Parameters<typeof mockApi>[0] {
  return {
    "GET /api/v1/sites/launch-page": { body: { site } },
    "GET /api/v1/me": { body: { user: alex } },
    "GET /api/v1/workspaces": { body: { workspaces: [personal] } },
    "GET /api/v1/workspaces/personal/folders": { body: { folders: [reportsFolder] } },
    "GET /api/v1/publish/launch-page/versions": {
      body: { versions: [{ id: "v1", siteId: "s1", status: "finalized", live: true, createdBy: "u1", createdAt: "2026-03-01T10:00:00Z", finalizedAt: "2026-03-01T10:01:00Z", fileCount: 3, totalBytes: 2048, baseVersionId: null }] },
    },
    ...extra,
  };
}

function renderSite(route = "/sites/launch-page"): void {
  renderWithApp(
    <Routes>
      <Route path="/sites/:slug" element={<SiteLayout />}>
        <Route index element={<OverviewTab />} />
      </Route>
      <Route path="/" element={<p>Home</p>} />
      <Route path="/workspaces/:workspace" element={<p>Workspace page</p>} />
    </Routes>,
    route,
  );
}

describe("site detail overview", () => {
  it("shows the header facts, the live version and a quiet notice", async () => {
    mockApi(routes());
    renderSite();
    expect(await screen.findByRole("heading", { name: "Launch page", level: 1 })).toBeInTheDocument();
    expect(await screen.findByText(/3 files, 2 KB/)).toBeInTheDocument();
    expect(screen.getByText("The subdomain address for this site is switched off.")).toBeInTheDocument();
    expect(document.title).toBe("Launch page | OpenHere");
  });

  it("shows the same not-found page as the API for an unknown site", async () => {
    mockApi({ ...routes(), "GET /api/v1/sites/launch-page": apiError(404, "not_found", "Not found.") });
    renderSite();
    expect(await screen.findByRole("heading", { name: "Site not found" })).toBeInTheDocument();
  });

  it("saves only the changed detail fields", async () => {
    const fetchMock = mockApi(routes(makeSite(), { "PATCH /api/v1/sites/launch-page": { body: { site: makeSite({ displayName: "Launch page 2" }) } } }));
    renderSite();
    const name = await screen.findByLabelText("Display name");
    const save = screen.getByRole("button", { name: "Save changes" });
    expect(save).toBeDisabled();
    await userEvent.clear(name);
    await userEvent.type(name, "Launch page 2");
    await userEvent.click(save);
    await waitFor(() => expect(screen.getByText("Site details saved.")).toBeInTheDocument());
    const patch = fetchMock.mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === "PATCH") as [string, RequestInit];
    expect(JSON.parse(String(patch[1].body))).toEqual({ displayName: "Launch page 2" });
  });

  it("toggles single-page mode immediately", async () => {
    const fetchMock = mockApi(routes(makeSite(), { "PATCH /api/v1/sites/launch-page": { body: { site: makeSite({ spaMode: true }) } } }));
    renderSite();
    await userEvent.click(await screen.findByRole("switch", { name: "Single-page app mode" }));
    await waitFor(() => expect(screen.getByText("Single-page app mode is on.")).toBeInTheDocument());
    const patch = fetchMock.mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === "PATCH") as [string, RequestInit];
    expect(JSON.parse(String(patch[1].body))).toEqual({ spaMode: true });
  });

  it("shows read-only users the values and no controls", async () => {
    mockApi(routes(makeSite({ role: "read" })));
    renderSite();
    await screen.findByRole("heading", { name: "Launch page", level: 1 });
    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete this site" })).not.toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "Count page views" })).toBeDisabled();
    expect(screen.getByText(/read-only access to this site, so you cannot change these fields/)).toBeInTheDocument();
  });

  it("deletes the site only after typing its slug", async () => {
    mockApi(routes(makeSite(), { "DELETE /api/v1/sites/launch-page": { status: 204 } }));
    renderSite();
    await userEvent.click(await screen.findByRole("button", { name: "Delete this site" }));
    const confirm = await screen.findByRole("button", { name: "Delete site" });
    expect(confirm).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Type launch-page to confirm"), "launch-page");
    await userEvent.click(confirm);
    expect(await screen.findByText("Workspace page")).toBeInTheDocument();
  });

  it("shows the cause and settings link to root", async () => {
    const base = makeSite();
    const site = makeSite({ hosting: { ...base.hosting, modes: { ...base.hosting.modes, subdomain: { state: "off", reason: "wildcard_check_failed" } } } });
    mockApi(
      routes(site, {
        "GET /api/v1/hosting/status": {
          body: {
            degraded: false,
            modes: site.hosting.modes,
            notices: [],
            detail: { wildcardStatus: "failed", wildcardCheckedAt: null, wildcardDetail: "no answer from the probe", consecutiveFailures: 3, subdomainDisabledReason: "wildcard_check_failed", tls: {} },
          },
        },
      }),
    );
    renderSite();
    expect(await screen.findByText(/no answer from the probe/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open hosting settings" })).toHaveAttribute("href", "/admin/hosting");
  });
});
