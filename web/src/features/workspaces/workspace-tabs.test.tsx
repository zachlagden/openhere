import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Workspace } from "@/api/types";
import { acme, alex, instanceInfo, makeReport, personal } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { WorkspaceAnalyticsTab } from "./workspace-analytics-tab";
import { WorkspaceLayout } from "./workspace-layout";
import { WorkspaceSettingsTab } from "./workspace-settings-tab";

afterEach(() => {
  vi.unstubAllGlobals();
});

function setup(path: string, workspace: Workspace, extra: Parameters<typeof mockApi>[0] = {}): ReturnType<typeof mockApi> {
  const fetchMock = mockApi({
    "GET /api/v1/me": { body: { user: alex } },
    "GET /api/v1/instance": { body: instanceInfo },
    "GET /api/v1/workspaces": () => ({ body: { workspaces: [personal, workspace] } }),
    "GET /api/v1/workspaces/acme/analytics": { body: makeReport() },
    ...extra,
  });
  renderWithApp(
    <Routes>
      <Route path="/workspaces/:workspace" element={<WorkspaceLayout />}>
        <Route path="analytics" element={<WorkspaceAnalyticsTab />} />
        <Route path="settings" element={<WorkspaceSettingsTab />} />
      </Route>
      <Route path="/" element={<p>Home page</p>} />
    </Routes>,
    path,
  );
  return fetchMock;
}

describe("workspace analytics tab", () => {
  it("shows the aggregate report to workspace-wide users", async () => {
    setup("/workspaces/acme/analytics?range=7d", acme);
    expect(await screen.findByTestId("total-views")).toHaveTextContent("30");
  });

  it("shows a clear denial to scoped-only users without calling the API", async () => {
    const fetchMock = setup("/workspaces/acme/analytics", { ...acme, role: "read", scopedOnly: true });
    expect(await screen.findByText("Workspace analytics is not available to you.")).toBeInTheDocument();
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes("/analytics"))).toBe(false);
  });

  it("shows the server's message if it denies a request", async () => {
    setup("/workspaces/acme/analytics?range=7d", acme, { "GET /api/v1/workspaces/acme/analytics": apiError(403, "forbidden", "You need access to the whole workspace.") });
    expect(await screen.findByText("You need access to the whole workspace.")).toBeInTheDocument();
  });
});

describe("workspace settings tab", () => {
  it("renames only when the name changed", async () => {
    const fetchMock = setup("/workspaces/acme/settings", acme, { "PATCH /api/v1/workspaces/acme": { body: { workspace: { ...acme, name: "Acme Group" } } } });
    const field = await screen.findByLabelText("Name");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    await userEvent.clear(field);
    await userEvent.type(field, "Acme Group");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(screen.getByText("Workspace renamed.")).toBeInTheDocument());
    const patch = fetchMock.mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === "PATCH") as [string, RequestInit];
    expect(JSON.parse(String(patch[1].body))).toEqual({ name: "Acme Group" });
  });

  it("explains the must-be-empty rule when deletion is refused", async () => {
    setup("/workspaces/acme/settings", acme, { "DELETE /api/v1/workspaces/acme": apiError(409, "workspace_not_empty", "The workspace still has sites.") });
    expect(await screen.findByText(/only be deleted when it holds no sites/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Delete this workspace" }));
    await userEvent.type(await screen.findByLabelText("Type acme to confirm"), "acme");
    await userEvent.click(screen.getByRole("button", { name: "Delete workspace" }));
    expect(await screen.findByText("This workspace still contains sites. Delete them or ask their owners to, then try again.")).toBeInTheDocument();
  });

  it("returns home after deleting an empty workspace", async () => {
    setup("/workspaces/acme/settings", acme, { "DELETE /api/v1/workspaces/acme": { status: 204 } });
    await userEvent.click(await screen.findByRole("button", { name: "Delete this workspace" }));
    await userEvent.type(await screen.findByLabelText("Type acme to confirm"), "acme");
    await userEvent.click(screen.getByRole("button", { name: "Delete workspace" }));
    expect(await screen.findByText("Home page")).toBeInTheDocument();
  });

  it("says a personal workspace cannot be deleted", async () => {
    mockApi({
      "GET /api/v1/me": { body: { user: alex } },
      "GET /api/v1/instance": { body: instanceInfo },
      "GET /api/v1/workspaces": { body: { workspaces: [personal] } },
    });
    renderWithApp(
      <Routes>
        <Route path="/workspaces/:workspace" element={<WorkspaceLayout />}>
          <Route path="settings" element={<WorkspaceSettingsTab />} />
        </Route>
      </Routes>,
      "/workspaces/personal/settings",
    );
    expect(await screen.findByText(/cannot be deleted/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete this workspace" })).not.toBeInTheDocument();
  });

  it("tells non-admins they cannot change settings", async () => {
    setup("/workspaces/acme/settings", { ...acme, role: "write" });
    expect(await screen.findByText("Only workspace admins can change these settings.")).toBeInTheDocument();
  });
});
