import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Version } from "@/api/types";
import { alex, makeSite, personal } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { SiteLayout } from "./site-layout";
import { VersionsTab } from "./versions-tab";

afterEach(() => {
  vi.unstubAllGlobals();
});

function version(overrides: Partial<Version>): Version {
  return {
    id: "v-live-0001",
    siteId: "s1",
    status: "finalized",
    live: false,
    createdBy: "u1",
    createdAt: "2026-03-01T10:00:00Z",
    finalizedAt: "2026-03-01T10:01:00Z",
    fileCount: 3,
    totalBytes: 2048,
    baseVersionId: null,
    ...overrides,
  };
}

const history: Version[] = [
  version({ id: "v-live-0001", live: true }),
  version({ id: "v-old-00002", createdBy: "u2", fileCount: 1, totalBytes: 100 }),
  version({ id: "v-pend-0003", status: "pending", finalizedAt: null }),
];

function setup(role: "read" | "write" | "admin" = "admin", extra: Parameters<typeof mockApi>[0] = {}): ReturnType<typeof mockApi> {
  const fetchMock = mockApi({
    "GET /api/v1/sites/launch-page": { body: { site: makeSite({ role }) } },
    "GET /api/v1/me": { body: { user: alex } },
    "GET /api/v1/workspaces": { body: { workspaces: [personal] } },
    "GET /api/v1/workspaces/personal/folders": { body: { folders: [] } },
    "GET /api/v1/publish/launch-page/versions": { body: { versions: history } },
    "GET /api/v1/sites/launch-page/files": {
      body: { versionId: "v-live-0001", files: [{ path: "index.html", size: 120, sha256: "a", contentType: "text/html" }, { path: "assets/app.js", size: 4096, sha256: "b", contentType: "text/javascript" }] },
    },
    ...extra,
  });
  renderWithApp(
    <Routes>
      <Route path="/sites/:slug" element={<SiteLayout />}>
        <Route path="versions" element={<VersionsTab />} />
      </Route>
    </Routes>,
    "/sites/launch-page/versions",
  );
  return fetchMock;
}

describe("versions tab", () => {
  it("lists versions with live and unfinished badges and no way to upload", async () => {
    setup();
    const table = await screen.findByRole("table", { name: /Versions of this site/ });
    const rows = within(table).getAllByRole("row");
    expect(rows).toHaveLength(4);
    expect(within(rows[1]!).getByText("Live")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("You")).toBeInTheDocument();
    expect(within(rows[2]!).getByText("Another member")).toBeInTheDocument();
    expect(within(rows[3]!).getByText("Not finished")).toBeInTheDocument();
    expect(screen.queryByText(/upload a/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/choose files/i)).not.toBeInTheDocument();
  });

  it("offers restore only for finalized versions that are not live, and delete for any non-live one", async () => {
    setup();
    await screen.findByRole("table");
    expect(screen.queryByRole("button", { name: "Restore version v-live-0" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Restore version v-old-00" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Restore version v-pend-0" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete version v-pend-0" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete version v-live-0" })).not.toBeInTheDocument();
  });

  it("restores after confirming", async () => {
    const fetchMock = setup("admin", {
      "POST /api/v1/publish/launch-page/versions/v-old-00002/restore": { body: { slug: "launch-page", url: null, hosting: makeSite().hosting, version: version({ id: "v-old-00002", live: true }) } },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Restore version v-old-00" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Restore version" }));
    await waitFor(() => expect(screen.getByText("Version v-old-00 is live again.")).toBeInTheDocument());
    expect(fetchMock.mock.calls.some(([url, init]) => String(url).endsWith("/restore") && (init as RequestInit).method === "POST")).toBe(true);
  });

  it("shows the API error when deleting fails", async () => {
    setup("admin", { "DELETE /api/v1/publish/launch-page/versions/v-old-00002": apiError(409, "version_live", "The live version cannot be deleted.") });
    await userEvent.click(await screen.findByRole("button", { name: "Delete version v-old-00" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete version" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("The live version cannot be deleted.");
  });

  it("lists live files and filters them", async () => {
    setup();
    expect(await screen.findByText("assets/app.js")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Filter files"), "index");
    expect(screen.queryByText("assets/app.js")).not.toBeInTheDocument();
    expect(screen.getByText("index.html")).toBeInTheDocument();
  });

  it("hides actions from read-only users", async () => {
    setup("read");
    await screen.findByRole("table");
    expect(screen.queryByRole("button", { name: /Restore version/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Delete version/ })).not.toBeInTheDocument();
  });
});
