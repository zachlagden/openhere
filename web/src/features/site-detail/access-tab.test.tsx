import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Access } from "@/api/types";
import { alex, makeSite, personal } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { AccessTab } from "./access-tab";
import { SiteLayout } from "./site-layout";

afterEach(() => {
  vi.unstubAllGlobals();
});

const publicAccess: Access = { mode: "public", hasPassword: false, emailAllowlist: [], domainAllowlist: [], emailCodeAvailable: true };

function setup(initial: Access, siteOverrides = {}, user = alex, extra: Parameters<typeof mockApi>[0] = {}): ReturnType<typeof mockApi> {
  let access = initial;
  const patch = extra["PATCH /api/v1/publish/launch-page/access"];
  const fetchMock = mockApi({
    "GET /api/v1/sites/launch-page": { body: { site: makeSite(siteOverrides) } },
    "GET /api/v1/me": { body: { user } },
    "GET /api/v1/workspaces": { body: { workspaces: [personal] } },
    "GET /api/v1/workspaces/personal/folders": { body: { folders: [] } },
    "GET /api/v1/publish/launch-page/versions": { body: { versions: [] } },
    "GET /api/v1/publish/launch-page/access": () => ({ body: { access } }),
    ...extra,
    ...(patch === undefined
      ? {}
      : {
          "PATCH /api/v1/publish/launch-page/access": (request) => {
            const answer = typeof patch === "function" ? patch(request) : patch;
            if ((answer.status ?? 200) === 200) {
              access = (answer.body as { access: Access }).access;
            }
            return answer;
          },
        }),
  });
  renderWithApp(
    <Routes>
      <Route path="/sites/:slug" element={<SiteLayout />}>
        <Route path="access" element={<AccessTab />} />
      </Route>
    </Routes>,
    "/sites/launch-page/access",
  );
  return fetchMock;
}

function patchBodies(fetchMock: ReturnType<typeof mockApi>): unknown[] {
  return fetchMock.mock.calls.filter(([, init]) => (init as RequestInit | undefined)?.method === "PATCH").map(([, init]) => JSON.parse(String((init as RequestInit).body)));
}

describe("access tab", () => {
  it("sets a password and sends it once without showing the stored one", async () => {
    const saved: Access = { ...publicAccess, mode: "password", hasPassword: true };
    const fetchMock = setup(publicAccess, {}, alex, { "PATCH /api/v1/publish/launch-page/access": { body: { access: saved } } });
    await userEvent.click(await screen.findByRole("radio", { name: /Password/ }));
    expect(await screen.findByText("Set a password before switching this site to password mode.")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Password", { selector: "input" }), "a long example password");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(screen.getByText("Access settings saved.")).toBeInTheDocument());
    expect(patchBodies(fetchMock)).toEqual([{ mode: "password", password: "a long example password" }]);
    expect(await screen.findByText("A password is set")).toBeInTheDocument();
    expect(document.body.innerHTML).not.toContain("a long example password");
  });

  it("clears the password after confirming", async () => {
    const fetchMock = setup({ ...publicAccess, hasPassword: true }, {}, alex, {
      "PATCH /api/v1/publish/launch-page/access": { body: { access: publicAccess } },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Remove password" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Remove password" }));
    await waitFor(() => expect(screen.getByText("Password removed.")).toBeInTheDocument());
    expect(patchBodies(fetchMock)).toEqual([{ password: null }]);
  });

  it("adds and removes allowlist entries and validates them", async () => {
    const fetchMock = setup({ ...publicAccess, mode: "restricted" }, {}, alex, {
      "PATCH /api/v1/publish/launch-page/access": { body: { access: { ...publicAccess, mode: "restricted", emailAllowlist: ["sam@example.com"] } } },
    });
    const field = await screen.findByLabelText("Add an email address");
    await userEvent.type(field, "not-an-email");
    await userEvent.click(screen.getAllByRole("button", { name: "Add" })[0]!);
    expect(await screen.findByText("Enter a plain email address such as sam@example.com.")).toBeInTheDocument();
    await userEvent.clear(field);
    await userEvent.type(field, "Sam@Example.com");
    await userEvent.click(screen.getAllByRole("button", { name: "Add" })[0]!);
    expect(await screen.findByRole("button", { name: "Remove sam@example.com" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(patchBodies(fetchMock)).toEqual([{ emailAllowlist: ["sam@example.com"] }]));
  });

  it("explains missing email to root with a link and to others as plain text", async () => {
    setup({ ...publicAccess, emailCodeAvailable: false });
    const link = await screen.findByRole("link", { name: "Set up email" });
    expect(link).toHaveAttribute("href", "/admin/email");
  });

  it("gives non-root users the note without a link", async () => {
    setup({ ...publicAccess, emailCodeAvailable: false }, {}, { ...alex, isRoot: false });
    expect(await screen.findByText("Ask the server administrator to set up email.")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Set up email" })).not.toBeInTheDocument();
  });

  it("shows read-only users the values without controls", async () => {
    setup({ ...publicAccess, mode: "restricted", emailAllowlist: ["sam@example.com"], hasPassword: true }, { role: "read" });
    expect(await screen.findByText("sam@example.com")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Remove sam@example.com" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("New password")).not.toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Restricted/ })).toBeDisabled();
  });

  it("shows the API error when the save is rejected", async () => {
    setup(publicAccess, {}, alex, { "PATCH /api/v1/publish/launch-page/access": apiError(400, "validation_failed", "Email allowlist entry 1 is not a valid email address.") });
    await userEvent.click(await screen.findByRole("radio", { name: /Restricted/ }));
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Email allowlist entry 1 is not a valid email address.");
  });
});
