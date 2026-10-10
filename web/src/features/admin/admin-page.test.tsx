import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { HostingStatus, Settings } from "@/api/types";
import { alex, instanceInfo, makeSettings, personal } from "@/test/fixtures";
import { apiError, mockApi, type Handler } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { AdminPage } from "./admin-page";

afterEach(() => {
  vi.unstubAllGlobals();
});

const hostingStatus: HostingStatus = {
  degraded: false,
  modes: { subdomain: { state: "off", reason: "switched_off" }, path: { state: "on", reason: null }, customDomains: { state: "on", reason: null } },
  notices: [],
  detail: {
    wildcardStatus: "unchecked",
    wildcardCheckedAt: null,
    wildcardDetail: "",
    consecutiveFailures: 0,
    subdomainDisabledReason: null,
    tls: {
      enabled: true,
      managementCertificateExpiresAt: "2026-06-01T10:00:00Z",
      wildcard: { state: "none", reason: "No DNS provider is configured." },
      customDomains: { count: 2, soonestExpiresAt: "2026-05-01T10:00:00Z" },
      dnsProviderConfigured: false,
    },
  },
};

interface Setup {
  settings?: Settings;
  user?: typeof alex;
  routes?: Record<string, Handler>;
}

function setup(path: string, options: Setup = {}): ReturnType<typeof mockApi> {
  let current = options.settings ?? makeSettings();
  const fetchMock = mockApi({
    "GET /api/v1/me": { body: { user: options.user ?? alex } },
    "GET /api/v1/instance": { body: instanceInfo },
    "GET /api/v1/workspaces": { body: { workspaces: [personal] } },
    "GET /api/v1/admin/settings": () => ({ body: { settings: current } }),
    "GET /api/v1/hosting/status": { body: hostingStatus },
    "PATCH /api/v1/admin/settings": (request) => {
      current = { ...current, ...((request.body as { settings: Partial<Settings> }).settings) };
      return { body: { settings: current, notices: [] } };
    },
    ...options.routes,
  });
  renderWithApp(
    <Routes>
      <Route path="/admin/*" element={<AdminPage />} />
    </Routes>,
    path,
  );
  return fetchMock;
}

function patchBodies(fetchMock: ReturnType<typeof mockApi>): { settings: Record<string, unknown> }[] {
  return fetchMock.mock.calls
    .filter(([url, init]) => String(url).includes("/admin/settings") && (init as RequestInit | undefined)?.method === "PATCH")
    .map(([, init]) => JSON.parse(String((init as RequestInit).body)) as { settings: Record<string, unknown> });
}

describe("admin access", () => {
  it("shows the not-found page to people who are not root", async () => {
    setup("/admin/hosting", { user: { ...alex, isRoot: false } });
    expect(await screen.findByRole("heading", { name: "Page not found" })).toBeInTheDocument();
    expect(screen.queryByText("Instance admin")).not.toBeInTheDocument();
  });

  it("opens on the hosting section", async () => {
    setup("/admin");
    expect(await screen.findByRole("heading", { name: "Site addresses" })).toBeInTheDocument();
  });
});

describe("hosting", () => {
  it("disables Save until a field changes, then sends only the changed key", async () => {
    const fetchMock = setup("/admin/hosting");
    const form = await screen.findByRole("form", { name: "Custom domains" });
    const save = within(form).getByRole("button", { name: "Save changes" });
    expect(save).toBeDisabled();
    const field = within(form).getByLabelText(/Domains per site/);
    await userEvent.clear(field);
    await userEvent.type(field, "4");
    expect(save).toBeEnabled();
    await userEvent.click(save);
    await waitFor(() => expect(screen.getByText("Settings saved.")).toBeInTheDocument());
    expect(patchBodies(fetchMock)).toEqual([{ settings: { "hosting.maxDomainsPerSite": 4 } }]);
  });

  it("will not let subdomain addresses be switched on before the wildcard check passes", async () => {
    setup("/admin/hosting");
    expect(await screen.findByRole("switch", { name: "Subdomain addresses" })).toBeDisabled();
  });

  it("allows it once the check passes", async () => {
    setup("/admin/hosting", { settings: makeSettings({ "hosting.wildcardStatus": "ok" }) });
    expect(await screen.findByRole("switch", { name: "Subdomain addresses" })).toBeEnabled();
  });

  it("explains the never-both-off rule from a 409", async () => {
    setup("/admin/hosting", {
      routes: { "PATCH /api/v1/admin/settings": apiError(409, "no_hosting_mode", "At least one hosting mode must stay on.") },
    });
    const form = await screen.findByRole("form", { name: "Site addresses" });
    await userEvent.click(within(form).getByRole("switch", { name: "Path addresses" }));
    await userEvent.click(within(form).getByRole("button", { name: "Save changes" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/Path addresses stop working/)).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Save changes" }));
    expect(await within(form).findByText(/must stay on, or no site could be reached/)).toBeInTheDocument();
  });

  it("confirms a base domain change and shows the notice the server returns", async () => {
    setup("/admin/hosting", {
      routes: {
        "PATCH /api/v1/admin/settings": () => ({ body: { settings: makeSettings({ "hosting.baseDomain": "sites.example.org", "hosting.pathEnabled": true }), notices: [{ code: "path_mode_enabled", message: "Path addresses were switched on so sites stay reachable." }] } }),
      },
    });
    const form = await screen.findByRole("form", { name: "Site addresses" });
    const field = within(form).getByLabelText(/Base domain/);
    await userEvent.clear(field);
    await userEvent.type(field, "sites.example.org");
    await userEvent.click(within(form).getByRole("button", { name: "Save changes" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/resets the wildcard check/)).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText("Path addresses were switched on so sites stay reachable.")).toBeInTheDocument();
  });

  it("runs the wildcard check and reports a failure plainly", async () => {
    setup("/admin/hosting", {
      routes: {
        "POST /api/v1/admin/hosting/check": () => ({
          body: { wildcard: { status: "failed", checkedAt: "2026-03-01T10:00:00Z", detail: "no DNS answer for the random label" }, settings: makeSettings({ "hosting.wildcardStatus": "failed", "hosting.wildcardDetail": "no DNS answer for the random label", "hosting.wildcardFailures": 1 }) },
        }),
      },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Check now" }));
    expect(await screen.findByText(/The check failed\. no DNS answer for the random label/)).toBeInTheDocument();
  });

  it("shows the certificate state from the root-only detail", async () => {
    setup("/admin/hosting");
    expect(await screen.findByText("2 loaded")).toBeInTheDocument();
    expect(screen.getByText("No DNS provider is configured.")).toBeInTheDocument();
  });

  it("never renders the Cloudflare token and shows only whether it is set", async () => {
    setup("/admin/hosting", { settings: makeSettings({ "tls.dnsProvider": "cloudflare", "tls.cloudflareApiTokenSet": true }) });
    expect(await screen.findByText("Set")).toBeInTheDocument();
    const field = screen.getByLabelText("Replace with a new value");
    expect(field).toHaveValue("");
    expect(screen.getByRole("button", { name: "Clear cloudflare api token" })).toBeInTheDocument();
  });

  it("sends a new token once and clears it from the field", async () => {
    const fetchMock = setup("/admin/hosting");
    const form = await screen.findByRole("form", { name: "DNS provider" });
    await userEvent.type(within(form).getByLabelText("Value"), "cf-example-token-123");
    await userEvent.click(within(form).getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(screen.getByText("Settings saved.")).toBeInTheDocument());
    expect(patchBodies(fetchMock)).toEqual([{ settings: { "tls.cloudflareApiToken": "cf-example-token-123" } }]);
    expect(document.body.innerHTML).not.toContain("cf-example-token-123");
  });
});

describe("email", () => {
  it("shows provider fields, hides the password and reports a failed test plainly", async () => {
    setup("/admin/email", {
      settings: makeSettings({ "email.provider": "smtp", "email.fromAddress": "noreply@example.com", "email.smtp.host": "smtp.example.com", "email.smtp.passwordSet": true }),
      routes: { "POST /api/v1/admin/email/test": apiError(502, "email_send_failed", "The server rejected the username or password.") },
    });
    expect(await screen.findByDisplayValue("smtp.example.com")).toBeInTheDocument();
    expect(screen.getByText("Set")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("hunter2")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Send test email" }));
    expect(await screen.findByText("The server rejected the username or password.")).toBeInTheDocument();
  });

  it("reports a successful test", async () => {
    setup("/admin/email", { routes: { "POST /api/v1/admin/email/test": { body: { sent: true, to: "alex@example.com" } } } });
    await userEvent.click(await screen.findByRole("button", { name: "Send test email" }));
    expect(await screen.findByText(/handed to the provider for alex@example.com/)).toBeInTheDocument();
  });

  it("shows a per-field error from the API", async () => {
    setup("/admin/email", {
      settings: makeSettings({ "email.provider": "smtp" }),
      routes: { "PATCH /api/v1/admin/settings": apiError(400, "validation_failed", "Setting email.smtp.port must be between 1 and 65535.") },
    });
    const port = await screen.findByLabelText(/SMTP port/);
    await userEvent.clear(port);
    await userEvent.type(port, "99999");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    const message = await screen.findByText("Setting email.smtp.port must be between 1 and 65535.");
    expect(message.closest("div")).toContainElement(port);
  });

  it("confirms before turning email off", async () => {
    setup("/admin/email", { settings: makeSettings({ "email.provider": "resend", "email.fromAddress": "noreply@example.com" }) });
    await userEvent.selectOptions(await screen.findByLabelText("Provider"), "none");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText(/Invite emails, reset emails and sign-in codes stop working/)).toBeInTheDocument();
  });
});

describe("policy", () => {
  it("shows limits in sensible units and sends bytes", async () => {
    const fetchMock = setup("/admin/policy");
    const fileSize = await screen.findByLabelText("Size of one file");
    expect(fileSize).toHaveValue(256);
    expect(screen.getByLabelText("Size of one file unit")).toHaveValue("MB");
    expect(screen.getByLabelText("Size of one version")).toHaveValue(1);
    await userEvent.clear(fileSize);
    await userEvent.type(fileSize, "512");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(patchBodies(fetchMock)).toEqual([{ settings: { "limits.maxFileBytes": 536_870_912 } }]));
  });

  it("changes who can invite", async () => {
    const fetchMock = setup("/admin/policy");
    await userEvent.selectOptions(await screen.findByLabelText("Who can invite new people"), "workspaceAdmins");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(patchBodies(fetchMock)).toEqual([{ settings: { "invites.whoCanInvite": "workspaceAdmins" } }]));
  });
});

describe("users", () => {
  const sam = { id: "u2", email: "sam@example.com", displayName: "Sam", isRoot: false, disabled: false, disabledAt: null, createdAt: "2026-03-02T10:00:00Z" };
  const root = { id: "u1", email: "alex@example.com", displayName: "", isRoot: true, disabled: false, disabledAt: null, createdAt: "2026-03-01T10:00:00Z" };

  it("never offers to disable yourself", async () => {
    setup("/admin/users", { routes: { "GET /api/v1/admin/users": { body: { users: [root, sam] } } } });
    expect(await screen.findByRole("button", { name: "Disable sam@example.com" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Disable alex@example.com" })).not.toBeInTheDocument();
    expect(screen.getByText("This is you.")).toBeInTheDocument();
  });

  it("disables and re-enables an account after confirming", async () => {
    let disabled = false;
    setup("/admin/users", {
      routes: {
        "GET /api/v1/admin/users": () => ({ body: { users: [root, { ...sam, disabled }] } }),
        "PATCH /api/v1/admin/users/u2": (request) => {
          disabled = (request.body as { disabled: boolean }).disabled;
          return { body: { user: { ...sam, disabled } } };
        },
      },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Disable sam@example.com" }));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Disable account" }));
    expect(await screen.findByRole("button", { name: "Enable sam@example.com" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Enable sam@example.com" }));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Enable account" }));
    expect(await screen.findByRole("button", { name: "Disable sam@example.com" })).toBeInTheDocument();
  });

  it("explains the last-root rule", async () => {
    setup("/admin/users", {
      routes: {
        "GET /api/v1/admin/users": { body: { users: [root, { ...sam, isRoot: true }] } },
        "PATCH /api/v1/admin/users/u2": apiError(409, "last_root", "last root"),
      },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Disable sam@example.com" }));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Disable account" }));
    expect(await screen.findByText(/last root user/)).toBeInTheDocument();
  });

  it("shows a reset link once", async () => {
    setup("/admin/users", {
      routes: {
        "GET /api/v1/admin/users": { body: { users: [root, sam] } },
        "POST /api/v1/admin/users/u2/password-reset": { status: 201, body: { link: "https://sites.example.com/reset-password/reset-secret-789", expiresAt: "2026-03-02T11:00:00Z" } },
      },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Create a reset link for sam@example.com" }));
    expect(await screen.findByTestId("one-time-secret")).toHaveTextContent("reset-secret-789");
    await userEvent.click(screen.getByRole("button", { name: "I have saved it" }));
    expect(document.body.textContent).not.toContain("reset-secret-789");
  });

  it("invites a user and shows the link once", async () => {
    setup("/admin/users", {
      routes: {
        "GET /api/v1/admin/users": { body: { users: [root] } },
        "POST /api/v1/invites": { status: 201, body: { invite: { id: "i1", email: "kim@example.com", status: "pending", workspace: null, role: null, invitedBy: "alex@example.com", createdAt: "2026-03-01T10:00:00Z", expiresAt: "2026-03-08T10:00:00Z", acceptedAt: null, revokedAt: null }, link: "https://sites.example.com/invite/invite-secret-321", emailAvailable: false, emailSent: false, emailError: null } },
      },
    });
    await userEvent.type(await screen.findByLabelText("Email address"), "kim@example.com");
    await userEvent.click(screen.getByRole("button", { name: "Create invite" }));
    expect(await screen.findByTestId("one-time-secret")).toHaveTextContent("invite-secret-321");
    await userEvent.click(screen.getByRole("button", { name: "I have saved it" }));
    expect(document.body.textContent).not.toContain("invite-secret-321");
  });
});

describe("analytics", () => {
  it("never shows the MaxMind licence key", async () => {
    setup("/admin/analytics", { settings: makeSettings({ "analytics.maxmindAccountId": "123456", "analytics.maxmindLicenseKeySet": true, "analytics.geoStatus": "ok" }), routes: { "GET /api/v1/admin/analytics": { body: { from: "2026-03-01", to: "2026-03-03", interval: "day", totals: { views: 0, uniques: 0, botViews: 0 }, series: [], paths: [], referrers: [], countries: [], cities: [], browsers: [], operatingSystems: [], devices: [], statuses: [], locationAvailable: true, droppedEvents: 3, generatedAt: "2026-03-03T12:00:00Z" } } } });
    expect(await screen.findByDisplayValue("123456")).toBeInTheDocument();
    expect(screen.getByText("Working")).toBeInTheDocument();
    expect(screen.getByText("Set")).toBeInTheDocument();
    expect(await screen.findByText(/could not record since it started/)).toBeInTheDocument();
  });
});
