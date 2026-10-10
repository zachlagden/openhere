import { expectNoHorizontalScroll } from "./checks.mjs";

export async function tour(page, label, shot, problems) {
  const visit = async (route, ready, name) => {
    await page.goto(new URL(route, page.url()).toString());
    await ready();
    await shot(name);
    await expectNoHorizontalScroll(page, `${label} ${name}`, problems);
  };

  await visit("/", () => page.getByText("Launch page").first().waitFor(), "tour-sites");
  await visit("/workspaces/" + (await personalSlug(page)), async () => {
    if (page.viewportSize().width < 1024) {
      await page.getByRole("button", { name: "Folders" }).click();
    }
    await page.getByRole("link", { name: "Reports" }).waitFor();
  }, "tour-workspace");

  await page.getByRole("button", { name: "Account menu" }).click();
  await shot("tour-user-menu");
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: "Switch workspace" }).click();
  await shot("tour-switcher");
  await page.keyboard.press("Escape");

  await visit("/docs", () => page.getByRole("heading", { name: "Agent guide", level: 1 }).waitFor(), "tour-docs");
  await visit("/docs/api", () => page.getByRole("heading", { name: "Sites", level: 2 }).waitFor(), "tour-docs-api");
  await visit("/settings", () => page.getByRole("heading", { name: "Settings", level: 1 }).waitFor(), "tour-settings");
  for (const screen of SCREENS) {
    await visit(screen.route, () => screen.ready(page), screen.name);
  }
  await visit("/does-not-exist", () => page.getByRole("heading", { name: "Page not found" }).waitFor(), "tour-not-found");
  await visit("/invite/not-a-real-token", () => page.getByRole("heading", { name: "This invite is not valid" }).waitFor(), "tour-invite-invalid");
}

const heading = (name, options = {}) => (page) => page.getByRole("heading", { name, ...options }).first().waitFor();
const testId = (id) => (page) => page.getByTestId(id).waitFor();

const SCREENS = [
  { route: "/sites/launch-page", name: "tour-site-overview", ready: heading("Launch page (live)", { level: 1 }) },
  { route: "/sites/launch-page/access", name: "tour-site-access", ready: heading("Mode", { level: 2 }) },
  { route: "/sites/launch-page/domains", name: "tour-site-domains", ready: heading("Custom domains", { level: 2 }) },
  { route: "/sites/launch-page/analytics", name: "tour-site-analytics", ready: testId("total-views") },
  { route: "/sites/launch-page/versions", name: "tour-site-versions", ready: heading("Version history", { level: 2 }) },
  { route: "/workspaces/acme-ltd", name: "tour-acme-sites", ready: heading("Acme Ltd", { level: 1 }) },
  { route: "/workspaces/acme-ltd/members", name: "tour-acme-members", ready: heading("People with access", { level: 2 }) },
  { route: "/workspaces/acme-ltd/invites", name: "tour-acme-invites", ready: heading("Invite someone new", { level: 2 }) },
  { route: "/workspaces/acme-ltd/analytics", name: "tour-acme-analytics", ready: testId("total-views") },
  { route: "/workspaces/acme-ltd/settings", name: "tour-acme-settings", ready: heading("Delete workspace", { level: 2 }) },
  { route: "/settings/general", name: "tour-settings-general", ready: heading("Appearance", { level: 2 }) },
  { route: "/settings/api-keys", name: "tour-settings-keys", ready: heading("API keys", { level: 2 }) },
  { route: "/settings/analytics", name: "tour-settings-analytics", ready: testId("total-views") },
  { route: "/settings/domains", name: "tour-settings-domains", ready: heading("Custom domains", { level: 2 }) },
  { route: "/admin/hosting", name: "tour-admin-hosting", ready: heading("Site addresses", { level: 2 }) },
  { route: "/admin/email", name: "tour-admin-email", ready: heading("Send a test email", { level: 2 }) },
  { route: "/admin/users", name: "tour-admin-users", ready: heading("Accounts", { level: 2 }) },
  { route: "/admin/policy", name: "tour-admin-policy", ready: heading("Who can do what", { level: 2 }) },
  { route: "/admin/analytics", name: "tour-admin-analytics", ready: heading("Whole instance", { level: 2 }) },
];

async function personalSlug(page) {
  const response = await page.context().request.get(new URL("/api/v1/workspaces", page.url()).toString());
  const { workspaces } = await response.json();
  return workspaces.find((workspace) => workspace.kind === "personal").slug;
}
