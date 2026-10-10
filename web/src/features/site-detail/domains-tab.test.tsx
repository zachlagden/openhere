import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Domain } from "@/api/types";
import { alex, instanceInfo, makeDomain, makeSite, personal } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { DomainsTab } from "./domains-tab";
import { SiteLayout } from "./site-layout";

afterEach(() => {
  vi.unstubAllGlobals();
});

interface Options {
  domains?: Domain[];
  instance?: Partial<typeof instanceInfo>;
  role?: "read" | "write" | "admin";
  extra?: Parameters<typeof mockApi>[0];
}

function setup({ domains = [], instance = {}, role = "admin", extra = {} }: Options = {}): ReturnType<typeof mockApi> {
  let current = domains;
  const fetchMock = mockApi({
    "GET /api/v1/sites/launch-page": { body: { site: makeSite({ role }) } },
    "GET /api/v1/me": { body: { user: alex } },
    "GET /api/v1/workspaces": { body: { workspaces: [personal] } },
    "GET /api/v1/workspaces/personal/folders": { body: { folders: [] } },
    "GET /api/v1/publish/launch-page/versions": { body: { versions: [] } },
    "GET /api/v1/instance": { body: { ...instanceInfo, ...instance } },
    "GET /api/v1/sites/launch-page/domains": () => ({ body: { domains: current } }),
    "POST /api/v1/sites/launch-page/domains": (request) => {
      const created = makeDomain({ id: "d2", domain: (request.body as { domain: string }).domain });
      current = [...current, created];
      return { status: 201, body: { domain: created } };
    },
    ...extra,
  });
  renderWithApp(
    <Routes>
      <Route path="/sites/:slug" element={<SiteLayout />}>
        <Route path="domains" element={<DomainsTab />} />
      </Route>
    </Routes>,
    "/sites/launch-page/domains",
  );
  return fetchMock;
}

describe("domains tab", () => {
  it("adds a domain and shows its TXT challenge with copy buttons", async () => {
    setup();
    await userEvent.type(await screen.findByLabelText("Domain name"), "Docs.Example.org");
    await userEvent.click(screen.getByRole("button", { name: "Add domain" }));
    const card = await screen.findByTestId("domain-card");
    expect(within(card).getByRole("heading", { name: "docs.example.org" })).toBeInTheDocument();
    expect(within(card).getByText("Waiting for DNS")).toBeInTheDocument();
    expect(within(card).getByText("_selfhost-challenge.docs.example.org")).toBeInTheDocument();
    expect(within(card).getByText("ab12cd34ef56")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Copy TXT record value" })).toBeInTheDocument();
    expect(within(card).getByText("sites.example.com")).toBeInTheDocument();
  });

  it("shows the plain-language result of a failed check", async () => {
    setup({
      domains: [makeDomain()],
      extra: {
        "POST /api/v1/sites/launch-page/domains/d1/verify": {
          body: { domain: makeDomain({ lastCheckResult: "record_not_found" }), verified: false, message: "No TXT record was found at _selfhost-challenge.docs.example.org yet." },
        },
      },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Check DNS now" }));
    expect(await screen.findByText("No TXT record was found at _selfhost-challenge.docs.example.org yet.")).toBeInTheDocument();
  });

  it("offers primary and re-check for a verified domain and hides the challenge", async () => {
    setup({ domains: [makeDomain({ status: "verified", verifiedAt: "2026-03-02T10:00:00Z", lastCheckedAt: "2026-03-02T10:00:00Z", lastCheckResult: "verified" })] });
    const card = await screen.findByTestId("domain-card");
    expect(within(card).getByText("Verified")).toBeInTheDocument();
    expect(within(card).getByRole("button", { name: "Make primary" })).toBeInTheDocument();
    expect(within(card).queryByText("ab12cd34ef56")).not.toBeInTheDocument();
  });

  it("deletes a domain after confirming", async () => {
    const fetchMock = setup({ domains: [makeDomain()], extra: { "DELETE /api/v1/sites/launch-page/domains/d1": { status: 204 } } });
    await userEvent.click(await screen.findByRole("button", { name: /Delete/ }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete domain" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([, init]) => (init as RequestInit | undefined)?.method === "DELETE")).toBe(true));
  });

  it("explains when custom domains are switched off for the server", async () => {
    setup({ instance: { hostingModes: { subdomain: false, path: true, customDomains: false } } });
    expect(await screen.findByText(/Custom domains are switched off for this server/)).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Open hosting settings" })[0]).toHaveAttribute("href", "/admin/hosting");
  });

  it("shows the limit and blocks adding at the limit", async () => {
    setup({ instance: { maxDomainsPerSite: 1 }, domains: [makeDomain()] });
    expect(await screen.findByText(/1 of 1 used/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add domain" })).toBeDisabled();
    expect(screen.getByText(/most domains the server allows/)).toBeInTheDocument();
  });

  it("shows the API error when the domain is taken", async () => {
    setup({ extra: { "POST /api/v1/sites/launch-page/domains": apiError(409, "domain_taken", "That domain is already claimed by another site.") } });
    await userEvent.type(await screen.findByLabelText("Domain name"), "docs.example.org");
    await userEvent.click(screen.getByRole("button", { name: "Add domain" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("That domain is already claimed by another site.");
  });

  it("gives read-only users the list without actions", async () => {
    setup({ role: "read", domains: [makeDomain()] });
    await screen.findByTestId("domain-card");
    expect(screen.queryByRole("button", { name: "Check DNS now" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Domain name")).not.toBeInTheDocument();
  });
});
