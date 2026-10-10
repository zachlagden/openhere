import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeSite, personal } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { SiteList } from "./site-list";

afterEach(() => {
  vi.unstubAllGlobals();
});

const baseRoutes = {
  "GET /api/v1/workspaces/personal/folders": { body: { folders: [] } },
  "GET /api/v1/instance": { body: { publicUrl: "https://sites.example.com", emailAvailable: false, agentCredentialsPath: "~/.selfhost/credentials" } },
  "GET /api/v1/me": { body: { user: { email: "alex@example.com" } } },
};

function renderList(params = { q: "", folder: "" }, onClear = vi.fn()): void {
  renderWithApp(<SiteList workspace="" params={params} workspaces={[personal]} isRoot onClearFilters={onClear} />);
}

describe("SiteList", () => {
  it("shows a skeleton while loading", () => {
    mockApi({ ...baseRoutes, "GET /api/v1/sites": () => ({ body: { sites: [], nextCursor: null } }) });
    renderList();
    expect(screen.getByLabelText("Loading sites")).toBeInTheDocument();
  });

  it("shows the empty state with the prompt button", async () => {
    mockApi({ ...baseRoutes, "GET /api/v1/sites": { body: { sites: [], nextCursor: null } } });
    renderList();
    expect(await screen.findByRole("heading", { name: "No sites yet" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy agent setup prompt" })).toBeInTheDocument();
  });

  it("shows the no-match state and clears filters", async () => {
    mockApi({ ...baseRoutes, "GET /api/v1/sites": { body: { sites: [], nextCursor: null } } });
    const onClear = vi.fn();
    renderList({ q: "zzz", folder: "" }, onClear);
    await userEvent.click(await screen.findByRole("button", { name: "Clear search and filters" }));
    expect(onClear).toHaveBeenCalledOnce();
  });

  it("shows an error with a retry", async () => {
    mockApi({ ...baseRoutes, "GET /api/v1/sites": apiError(500, "internal_error", "Something went wrong.") });
    renderList();
    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong on the server.");
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
  });

  it("renders a site with its address, badges and hosting notice", async () => {
    mockApi({ ...baseRoutes, "GET /api/v1/sites": { body: { sites: [makeSite()], nextCursor: null } } });
    renderList();
    const row = await screen.findByTestId("site-row");
    expect(within(row).getByText("Launch page")).toBeInTheDocument();
    expect(within(row).getByText("sites.example.com/s/launch-page")).toBeInTheDocument();
    expect(within(row).getByText("Public")).toBeInTheDocument();
    expect(within(row).getByText("Analytics on")).toBeInTheDocument();
    expect(within(row).getByText("The subdomain address for this site is switched off.")).toBeInTheDocument();
    expect(within(row).getByRole("link", { name: "Open hosting settings" })).toHaveAttribute("href", "/settings/hosting");
  });

  it("hides the hosting settings link from non-root users", async () => {
    mockApi({ ...baseRoutes, "GET /api/v1/sites": { body: { sites: [makeSite()], nextCursor: null } } });
    renderWithApp(<SiteList workspace="" params={{ q: "", folder: "" }} workspaces={[personal]} isRoot={false} onClearFilters={() => undefined} />);
    await screen.findByTestId("site-row");
    expect(screen.queryByRole("link", { name: "Open hosting settings" })).not.toBeInTheDocument();
  });

  it("loads the next page with the cursor", async () => {
    const fetchMock = mockApi({
      ...baseRoutes,
      "GET /api/v1/sites": ({ search }) =>
        search.get("cursor") === "c1"
          ? { body: { sites: [makeSite({ id: "s2", slug: "second", displayName: "Second site" })], nextCursor: null } }
          : { body: { sites: [makeSite()], nextCursor: "c1" } },
    });
    renderList();
    await userEvent.click(await screen.findByRole("button", { name: "Load more sites" }));
    expect(await screen.findByText("Second site")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more sites" })).not.toBeInTheDocument();
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes("cursor=c1"))).toBe(true);
  });
});
