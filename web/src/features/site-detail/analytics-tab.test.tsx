import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { alex, makeReport, makeSite, personal } from "@/test/fixtures";
import { mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { AnalyticsTab } from "./analytics-tab";
import { SiteLayout } from "./site-layout";

afterEach(() => {
  vi.unstubAllGlobals();
});

function setup(options: { site?: Parameters<typeof makeSite>[0]; user?: typeof alex; report?: ReturnType<typeof makeReport> } = {}): ReturnType<typeof mockApi> {
  const fetchMock = mockApi({
    "GET /api/v1/sites/launch-page": { body: { site: makeSite(options.site) } },
    "GET /api/v1/me": { body: { user: options.user ?? alex } },
    "GET /api/v1/workspaces": { body: { workspaces: [personal] } },
    "GET /api/v1/workspaces/personal/folders": { body: { folders: [] } },
    "GET /api/v1/publish/launch-page/versions": { body: { versions: [] } },
    "GET /api/v1/sites/launch-page/analytics": { body: options.report ?? makeReport() },
    "PATCH /api/v1/sites/launch-page": { body: { site: makeSite({ analyticsEnabled: true }) } },
  });
  renderWithApp(
    <Routes>
      <Route path="/sites/:slug" element={<SiteLayout />}>
        <Route path="analytics" element={<AnalyticsTab />} />
      </Route>
    </Routes>,
    "/sites/launch-page/analytics?range=7d",
  );
  return fetchMock;
}

describe("analytics tab", () => {
  it("shows totals, bot views apart, the chart and the ranked lists", async () => {
    setup();
    expect(await screen.findByTestId("total-views")).toHaveTextContent("30");
    expect(screen.getByTestId("total-unique-visitors")).toHaveTextContent("18");
    expect(screen.getByTestId("total-bot-views")).toHaveTextContent("4");
    expect(screen.getByRole("img", { name: /3 points from/ })).toBeInTheDocument();
    const pages = screen.getByRole("region", { name: "Pages" });
    expect(within(pages).getByText("/pricing")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Devices" })).getByText("bot")).toBeInTheDocument();
  });

  it("requests the preset range and switches to hours for today", async () => {
    const fetchMock = setup();
    await screen.findByTestId("total-views");
    const first = String(fetchMock.mock.calls.find(([url]) => String(url).includes("/analytics"))?.[0]);
    expect(first).toMatch(/interval=day/);
    expect(first).toMatch(/top=10/);
    await userEvent.click(screen.getByRole("button", { name: "Today" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => String(url).includes("interval=hour"))).toBe(true));
  });

  it("rejects a custom range that is reversed", async () => {
    setup();
    await screen.findByTestId("total-views");
    fireEvent.change(screen.getByLabelText("From"), { target: { value: "2026-03-10" } });
    fireEvent.change(screen.getByLabelText("To"), { target: { value: "2026-03-01" } });
    await userEvent.click(screen.getByRole("button", { name: "Apply range" }));
    expect(await screen.findByText("The start date must not be after the end date.")).toBeInTheDocument();
  });

  it("explains missing location data to root with a settings link", async () => {
    setup();
    await screen.findByTestId("total-views");
    const countries = screen.getByRole("region", { name: "Countries" });
    expect(within(countries).getByRole("link", { name: "Add one in analytics settings" })).toHaveAttribute("href", "/admin/analytics");
  });

  it("explains missing location data to other users as plain text", async () => {
    setup({ user: { ...alex, isRoot: false } });
    await screen.findByTestId("total-views");
    const countries = screen.getByRole("region", { name: "Countries" });
    expect(within(countries).getByText(/Ask the administrator to add a MaxMind account/)).toBeInTheDocument();
    expect(within(countries).queryByRole("link")).not.toBeInTheDocument();
  });

  it("offers the data as a table", async () => {
    setup();
    await userEvent.click(await screen.findByRole("button", { name: "Show data table" }));
    const table = screen.getByRole("table", { name: /Views and unique visitors/ });
    expect(within(table).getAllByRole("row")).toHaveLength(4);
  });

  it("shows a tooltip when a point is focused with the keyboard", async () => {
    setup();
    const chart = await screen.findByRole("group", { name: /Use the left and right arrow keys/ });
    chart.focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(await screen.findByTestId("chart-tooltip")).toHaveTextContent("Sun, 1 Mar 2026");
    await userEvent.keyboard("{ArrowRight}");
    expect(screen.getByTestId("chart-tooltip")).toHaveTextContent("20");
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByTestId("chart-tooltip")).not.toBeInTheDocument();
  });

  it("shows an empty state with an enable button when analytics is off", async () => {
    const fetchMock = setup({ site: { analyticsEnabled: false }, report: makeReport({ totals: { views: 0, uniques: 0, botViews: 0 }, series: [] }) });
    expect(await screen.findByRole("heading", { name: "Analytics is off for this site" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Turn on analytics" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([, init]) => (init as RequestInit | undefined)?.method === "PATCH")).toBe(true));
  });

  it("hides the enable button from read-only users", async () => {
    setup({ site: { analyticsEnabled: false, role: "read" } });
    expect(await screen.findByText("Someone with write access can turn it on.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Turn on analytics" })).not.toBeInTheDocument();
  });

  it("says so when no views were recorded", async () => {
    setup({ report: makeReport({ totals: { views: 0, uniques: 0, botViews: 0 }, series: [] }) });
    expect(await screen.findByText("No page views were recorded in this range.")).toBeInTheDocument();
  });
});
