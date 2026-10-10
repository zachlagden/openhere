import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CreatedInvite, Invite, Workspace } from "@/api/types";
import { acme, alex, instanceInfo, personal } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { InvitesTab } from "./invites-tab";
import { WorkspaceLayout } from "./workspace-layout";

afterEach(() => {
  vi.unstubAllGlobals();
});

const pending: Invite = {
  id: "i1",
  email: "kim@example.com",
  status: "pending",
  workspace: { id: "w2", slug: "acme", name: "Acme Ltd" },
  role: "read",
  invitedBy: "alex@example.com",
  createdAt: "2026-03-01T10:00:00Z",
  expiresAt: "2026-03-08T10:00:00Z",
  acceptedAt: null,
  revokedAt: null,
};

function created(overrides: Partial<CreatedInvite> = {}): CreatedInvite {
  return { invite: pending, link: "https://sites.example.com/invite/secret-token-123", emailAvailable: false, emailSent: false, emailError: null, ...overrides };
}

function setup(options: { workspace?: Workspace; user?: typeof alex; instance?: Partial<typeof instanceInfo>; extra?: Parameters<typeof mockApi>[0] } = {}): ReturnType<typeof mockApi> {
  const fetchMock = mockApi({
    "GET /api/v1/me": { body: { user: options.user ?? alex } },
    "GET /api/v1/instance": { body: { ...instanceInfo, ...options.instance } },
    "GET /api/v1/workspaces": { body: { workspaces: [personal, options.workspace ?? acme] } },
    "GET /api/v1/invites": { body: { invites: [pending, { ...pending, id: "i2", email: "other@example.com", workspace: null }] } },
    ...options.extra,
  });
  renderWithApp(
    <Routes>
      <Route path="/workspaces/:workspace" element={<WorkspaceLayout />}>
        <Route path="invites" element={<InvitesTab />} />
      </Route>
    </Routes>,
    "/workspaces/acme/invites",
  );
  return fetchMock;
}

describe("invites tab", () => {
  it("lists only this workspace's invites", async () => {
    setup();
    const table = await screen.findByRole("table", { name: /Invites for this workspace/ });
    expect(within(table).getByText("kim@example.com")).toBeInTheDocument();
    expect(within(table).queryByText("other@example.com")).not.toBeInTheDocument();
  });

  it("creates an invite, shows the link once and removes it when the dialog closes", async () => {
    const fetchMock = setup({ extra: { "POST /api/v1/invites": { status: 201, body: created() } } });
    await userEvent.type(await screen.findByLabelText("Email address"), "kim@example.com");
    await userEvent.click(screen.getByRole("button", { name: "Create invite" }));
    expect(await screen.findByTestId("one-time-secret")).toHaveTextContent("https://sites.example.com/invite/secret-token-123");
    expect(screen.getByText("This server cannot send email, so send the link yourself.")).toBeInTheDocument();
    const post = fetchMock.mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === "POST") as [string, RequestInit];
    expect(JSON.parse(String(post[1].body))).toEqual({ email: "kim@example.com", workspace: "acme", role: "read", sendEmail: false });
    await userEvent.click(screen.getByRole("button", { name: "I have saved it" }));
    expect(document.body.textContent).not.toContain("secret-token-123");
  });

  it("hides the email option when the server cannot send email", async () => {
    setup();
    await screen.findByLabelText("Email address");
    expect(screen.queryByRole("switch", { name: "Email the link too" })).not.toBeInTheDocument();
  });

  it("offers the email option and sends it when email works", async () => {
    const fetchMock = setup({
      instance: { emailAvailable: true },
      extra: { "POST /api/v1/invites": { status: 201, body: created({ emailAvailable: true, emailSent: true }) } },
    });
    await userEvent.type(await screen.findByLabelText("Email address"), "kim@example.com");
    await userEvent.click(screen.getByRole("switch", { name: "Email the link too" }));
    await userEvent.click(screen.getByRole("button", { name: "Create invite" }));
    expect(await screen.findByText(/also emailed to kim@example.com/)).toBeInTheDocument();
    const post = fetchMock.mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === "POST") as [string, RequestInit];
    expect(JSON.parse(String(post[1].body)).sendEmail).toBe(true);
  });

  it("explains an address that already has an account", async () => {
    setup({ extra: { "POST /api/v1/invites": apiError(409, "email_registered", "That address has an account.") } });
    await userEvent.type(await screen.findByLabelText("Email address"), "kim@example.com");
    await userEvent.click(screen.getByRole("button", { name: "Create invite" }));
    expect(await screen.findByText(/Members tab instead/)).toBeInTheDocument();
  });

  it("revokes after confirming", async () => {
    const fetchMock = setup({ extra: { "DELETE /api/v1/invites/i1": { status: 204 } } });
    await userEvent.click(await screen.findByRole("button", { name: "Revoke invite to kim@example.com" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Revoke invite" }));
    await waitFor(() => expect(fetchMock.mock.calls.some(([, init]) => (init as RequestInit | undefined)?.method === "DELETE")).toBe(true));
  });

  it("resends and shows the new link", async () => {
    setup({ extra: { "POST /api/v1/invites/i1/resend": { status: 201, body: created({ link: "https://sites.example.com/invite/new-link-456" }) } } });
    await userEvent.click(await screen.findByRole("button", { name: "Resend invite to kim@example.com" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Create new link" }));
    expect(await screen.findByTestId("one-time-secret")).toHaveTextContent("new-link-456");
  });

  it("explains the policy to workspace admins when only root may invite", async () => {
    setup({ user: { ...alex, isRoot: false } });
    expect(await screen.findByText(/lets only the root user send invites/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Invites" })).not.toBeInTheDocument();
  });

  it("lets workspace admins invite when the server allows it", async () => {
    setup({ user: { ...alex, isRoot: false }, instance: { whoCanInvite: "workspaceAdmins" } });
    expect(await screen.findByRole("button", { name: "Create invite" })).toBeInTheDocument();
    expect(screen.getByText("You see the invites you sent.")).toBeInTheDocument();
  });
});
