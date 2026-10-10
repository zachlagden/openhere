import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Member, Workspace } from "@/api/types";
import { acme, alex, instanceInfo, makeMember, makeSite, personal, reportsFolder } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { MembersTab } from "./members-tab";
import { WorkspaceLayout } from "./workspace-layout";

afterEach(() => {
  vi.unstubAllGlobals();
});

const sam = makeMember({ id: "g2", userId: "u2", email: "sam@example.com", role: "read" });

function setup(options: { workspace?: Workspace; members?: Member[]; extra?: Parameters<typeof mockApi>[0] } = {}): ReturnType<typeof mockApi> {
  const workspace = options.workspace ?? acme;
  const fetchMock = mockApi({
    "GET /api/v1/me": { body: { user: alex } },
    "GET /api/v1/instance": { body: instanceInfo },
    "GET /api/v1/workspaces": { body: { workspaces: [personal, workspace] } },
    "GET /api/v1/workspaces/acme/members": { body: { members: options.members ?? [makeMember(), sam] } },
    "GET /api/v1/workspaces/acme/folders": { body: { folders: [{ ...reportsFolder, workspaceId: "w2" }] } },
    "GET /api/v1/sites": { body: { sites: [makeSite({ workspaceId: "w2", workspaceSlug: "acme" })], nextCursor: null } },
    ...options.extra,
  });
  renderWithApp(
    <Routes>
      <Route path="/workspaces/:workspace" element={<WorkspaceLayout />}>
        <Route path="members" element={<MembersTab />} />
      </Route>
    </Routes>,
    "/workspaces/acme/members",
  );
  return fetchMock;
}

function bodies(fetchMock: ReturnType<typeof mockApi>, method: string): unknown[] {
  return fetchMock.mock.calls.filter(([, init]) => (init as RequestInit | undefined)?.method === method).map(([, init]) => JSON.parse(String((init as RequestInit).body ?? "null")));
}

describe("members tab", () => {
  it("lists grants with scope and role, marking the signed-in user", async () => {
    setup({ members: [makeMember(), sam, makeMember({ id: "g3", userId: "u2", email: "sam@example.com", role: "write", scopeType: "folder", scopeId: "f1", scopeName: "Reports" })] });
    const table = await screen.findByRole("table", { name: /People with access/ });
    const rows = within(table).getAllByRole("row");
    expect(rows).toHaveLength(4);
    expect(within(rows[1]!).getByText("You")).toBeInTheDocument();
    expect(within(table).getByText("Folder: Reports")).toBeInTheDocument();
  });

  it("changes a role", async () => {
    const fetchMock = setup({ extra: { "PATCH /api/v1/workspaces/acme/members/g2": { body: { member: { ...sam, role: "write" } } } } });
    await userEvent.selectOptions(await screen.findByLabelText("Role of sam@example.com for Whole workspace"), "write");
    await waitFor(() => expect(bodies(fetchMock, "PATCH")).toEqual([{ role: "write" }]));
  });

  it("explains the last admin rule when the server refuses", async () => {
    setup({ extra: { "PATCH /api/v1/workspaces/acme/members/g1": apiError(409, "last_admin", "The last admin cannot be demoted.") } });
    await userEvent.selectOptions(await screen.findByLabelText("Role of alex@example.com for Whole workspace"), "read");
    expect(await screen.findByText("A workspace needs at least one admin. Make someone else an admin first.")).toBeInTheDocument();
  });

  it("adds a member scoped to one folder", async () => {
    const fetchMock = setup({
      extra: { "POST /api/v1/workspaces/acme/members": { status: 201, body: { member: makeMember({ id: "g9", email: "kim@example.com", role: "read", scopeType: "folder", scopeId: "f1", scopeName: "Reports" }) } } },
    });
    await userEvent.type(await screen.findByLabelText("Email address"), "kim@example.com");
    await userEvent.selectOptions(screen.getByLabelText("Access to"), "folder");
    await userEvent.selectOptions(await screen.findByLabelText("Folder"), "f1");
    await userEvent.click(screen.getByRole("button", { name: "Add member" }));
    await waitFor(() => expect(bodies(fetchMock, "POST")).toEqual([{ email: "kim@example.com", role: "read", scopeType: "folder", scopeId: "f1" }]));
  });

  it("asks for a target before adding a scoped grant", async () => {
    setup();
    await userEvent.type(await screen.findByLabelText("Email address"), "kim@example.com");
    await userEvent.selectOptions(screen.getByLabelText("Access to"), "site");
    await userEvent.click(screen.getByRole("button", { name: "Add member" }));
    expect(await screen.findByText("Choose a site.")).toBeInTheDocument();
  });

  it("tells the admin to invite someone who has no account", async () => {
    setup({ extra: { "POST /api/v1/workspaces/acme/members": apiError(404, "user_not_found", "No user has that email.") } });
    await userEvent.type(await screen.findByLabelText("Email address"), "new@example.com");
    await userEvent.click(screen.getByRole("button", { name: "Add member" }));
    expect(await screen.findByText(/Invite them to create one/)).toBeInTheDocument();
  });

  it("removes a grant after confirming", async () => {
    const fetchMock = setup({ extra: { "DELETE /api/v1/workspaces/acme/members/g2": { status: 204 } } });
    await userEvent.click(await screen.findByRole("button", { name: "Remove sam@example.com from Whole workspace" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Remove access" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([, init]) => (init as RequestInit | undefined)?.method === "DELETE")).toBe(true));
  });

  it("hides the members tab and explains the rule to non-admins", async () => {
    setup({ workspace: { ...acme, role: "write" } });
    expect(await screen.findByText("Only workspace admins can see and change members.")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Members" })).not.toBeInTheDocument();
  });
});
