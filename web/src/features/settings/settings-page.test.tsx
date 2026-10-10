import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ApiKey } from "@/api/types";
import { alex, instanceInfo, makeDomain, makeReport, makeSite, personal } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { SettingsPage } from "./settings-page";

afterEach(() => {
  vi.unstubAllGlobals();
});

const FULL_KEY = "sk_live_example_full_key_0123456789";

function key(overrides: Partial<ApiKey> = {}): ApiKey {
  return { id: "k1", label: "ci runner", prefix: "sk_live_ab", createdAt: "2026-03-01T10:00:00Z", lastUsedAt: null, revokedAt: null, ...overrides };
}

function setup(path: string, routes: Parameters<typeof mockApi>[0] = {}): ReturnType<typeof mockApi> {
  const fetchMock = mockApi({
    "GET /api/v1/me": { body: { user: alex } },
    "GET /api/v1/instance": { body: instanceInfo },
    "GET /api/v1/workspaces": { body: { workspaces: [personal] } },
    ...routes,
  });
  renderWithApp(
    <Routes>
      <Route path="/settings/*" element={<SettingsPage />} />
    </Routes>,
    path,
  );
  return fetchMock;
}

describe("profile and general", () => {
  it("shows the profile read-only and says what is not available", async () => {
    setup("/settings");
    expect(await screen.findByText("alex@example.com")).toBeInTheDocument();
    expect(screen.getByText(/no way to change your display name or email address yet/)).toBeInTheDocument();
    expect(screen.getByText(/ask the root user for a reset link/)).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("changes the theme", async () => {
    setup("/settings/general");
    await userEvent.click(await screen.findByRole("radio", { name: /Dark/ }));
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    await userEvent.click(screen.getByRole("radio", { name: /Light/ }));
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("shows a not-found page for an unknown section", async () => {
    setup("/settings/nope");
    expect(await screen.findByRole("heading", { name: "Page not found" })).toBeInTheDocument();
  });
});

describe("API keys", () => {
  it("creates a key, shows it once and never lists the full value", async () => {
    let keys = [key()];
    setup("/settings/api-keys", {
      "GET /api/v1/me/keys": () => ({ body: { keys } }),
      "POST /api/v1/me/keys": () => {
        const created = key({ id: "k2", label: "laptop", prefix: "sk_live_cd" });
        keys = [created, ...keys];
        return { status: 201, body: { key: FULL_KEY, apiKey: created } };
      },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Create key" }));
    await userEvent.type(await screen.findByLabelText("Label"), "laptop");
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Create key" }));
    expect(await screen.findByTestId("one-time-secret")).toHaveTextContent(FULL_KEY);
    expect(screen.getByText(/only time the full key is shown/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "I have saved it" }));
    await waitFor(() => expect(screen.getByText("laptop")).toBeInTheDocument());
    expect(document.body.textContent).not.toContain(FULL_KEY);
    expect(screen.getByText("sk_live_cd…")).toBeInTheDocument();
  });

  it("revokes a key after confirming", async () => {
    const fetchMock = setup("/settings/api-keys", {
      "GET /api/v1/me/keys": { body: { keys: [key(), key({ id: "k0", label: "old", revokedAt: "2026-03-02T10:00:00Z" })] } },
      "DELETE /api/v1/me/keys/k1": { status: 204 },
    });
    expect(await screen.findByText("Revoked")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Revoke key old" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Revoke key ci runner" }));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Revoke key" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([, init]) => (init as RequestInit | undefined)?.method === "DELETE")).toBe(true));
  });

  it("disables creating at the 50 key limit and explains it", async () => {
    const keys = Array.from({ length: 50 }, (_, index) => key({ id: `k${index}`, label: `key ${index}` }));
    setup("/settings/api-keys", { "GET /api/v1/me/keys": { body: { keys } } });
    expect(await screen.findByText(/50 of 50 active keys used/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create key" })).toBeDisabled();
  });

  it("shows the limit message if the server refuses", async () => {
    setup("/settings/api-keys", {
      "GET /api/v1/me/keys": { body: { keys: [] } },
      "POST /api/v1/me/keys": apiError(409, "api_key_limit_reached", "Too many keys."),
    });
    await userEvent.click(await screen.findByRole("button", { name: "Create key" }));
    await userEvent.type(await screen.findByLabelText("Label"), "x");
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Create key" }));
    expect(await screen.findByText(/already have 50 active keys/)).toBeInTheDocument();
  });

  it("asks for a label", async () => {
    setup("/settings/api-keys", { "GET /api/v1/me/keys": { body: { keys: [] } } });
    await userEvent.click(await screen.findByRole("button", { name: "Create key" }));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Create key" }));
    expect(await screen.findByText(/Give the key a label/)).toBeInTheDocument();
  });
});

describe("domains overview and analytics", () => {
  it("lists domains across sites with links to each site's Domains tab", async () => {
    setup("/settings/domains", {
      "GET /api/v1/sites": { body: { sites: [makeSite()], nextCursor: null } },
      "GET /api/v1/sites/launch-page/domains": { body: { domains: [makeDomain(), makeDomain({ id: "d2", domain: "ok.example.org", status: "verified" })] } },
    });
    const table = await screen.findByRole("table", { name: /Custom domains across your sites/ });
    expect(within(table).getAllByRole("link", { name: "Launch page" })[0]).toHaveAttribute("href", "/sites/launch-page/domains");
    expect(screen.getByText(/2 domains, 1 waiting for DNS/)).toBeInTheDocument();
  });

  it("shows the personal workspace analytics", async () => {
    setup("/settings/analytics?range=7d", { "GET /api/v1/workspaces/personal/analytics": { body: makeReport() } });
    expect(await screen.findByTestId("total-views")).toHaveTextContent("30");
  });
});
