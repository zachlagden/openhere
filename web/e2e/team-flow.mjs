import { check, expectNoHorizontalScroll } from "./checks.mjs";
import { closeSecret, openTab, readSecret, sessionApi, toast } from "./helpers.mjs";
import { apiCall, publishSite } from "./publish.mjs";

const SAM = { email: "sam@example.com", password: "a long example password for sam", name: "Sam" };

async function createWorkspace(ctx) {
  const { page, server, shot } = ctx;
  await page.goto(`${server.baseUrl}/`);
  await page.getByRole("button", { name: "Switch workspace" }).click();
  await page.getByRole("menuitem", { name: /Create shared workspace/ }).click();
  await page.getByRole("dialog").getByLabel("Name").fill("Acme Ltd");
  await page.getByRole("button", { name: "Create workspace" }).click();
  await page.getByRole("heading", { name: "Acme Ltd", level: 1 }).waitFor();
  await shot("workspace-created");
  await apiCall(server.baseUrl, ctx.key, "POST", "/workspaces/acme-ltd/folders", { name: "Reports" });
  await publishSite(server.baseUrl, ctx.key, {
    slug: "acme-reports-site",
    displayName: "Acme reports site",
    workspace: "acme-ltd",
    folder: "Reports",
    files: { "index.html": "<!doctype html><title>Reports</title><h1>Reports</h1>" },
  });
  await publishSite(server.baseUrl, ctx.key, {
    slug: "acme-private-site",
    displayName: "Acme private site",
    workspace: "acme-ltd",
    files: { "index.html": "<!doctype html><title>Private</title><h1>Private</h1>" },
  });
}

async function inviteFromAdmin(ctx) {
  const { page, server, shot } = ctx;
  await page.goto(`${server.baseUrl}/admin/users`);
  await page.getByRole("heading", { name: "Invite a user" }).waitFor();
  await page.getByLabel("Email address").fill(SAM.email);
  await page.getByRole("button", { name: "Create invite" }).click();
  await page.getByTestId("one-time-secret").waitFor();
  const link = await readSecret(page);
  check(link.includes("/invite/"), "the invite link points at the invite page");
  await shot("admin-invite-link");
  await closeSecret(page, link);
  return link;
}

async function acceptInvite(ctx, link) {
  const { browser, combo } = ctx;
  const context = await browser.newContext({ viewport: combo.viewport, colorScheme: combo.colorScheme });
  const page = await context.newPage();
  ctx.watch(page, "invitee");
  await page.goto(link);
  await page.getByRole("heading", { name: "Create your account" }).waitFor();
  await page.getByLabel("Name (optional)").fill(SAM.name);
  await page.getByLabel("Password", { exact: true }).fill(SAM.password);
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByRole("heading", { name: "Sites", level: 1 }).waitFor();
  return { context, page };
}

async function addScopedReader(ctx) {
  const { page, server, shot } = ctx;
  await page.goto(`${server.baseUrl}/workspaces/acme-ltd/members`);
  await page.getByRole("heading", { name: "People with access" }).waitFor();
  await page.getByLabel("Email address").fill(SAM.email);
  await page.getByLabel("Access to").selectOption("folder");
  await page.getByLabel("Folder", { exact: true }).selectOption({ label: "Reports" });
  await page.getByRole("button", { name: "Add member" }).click();
  await toast(page, `Added ${SAM.email}.`);
  await page.getByText("Folder: Reports").waitFor();
  await shot("workspace-members");
  await page.getByLabel("Email address").fill("nobody@example.com");
  await page.getByLabel("Access to").selectOption("workspace");
  await page.getByRole("button", { name: "Add member" }).click();
  await page.getByText(/Invite them to create one/).waitFor();
}

async function workspaceInvite(ctx) {
  const { page, server, shot } = ctx;
  await page.goto(`${server.baseUrl}/workspaces/acme-ltd/invites`);
  await page.getByLabel("Email address").fill("kim@example.com");
  await page.getByRole("button", { name: "Create invite" }).click();
  await page.getByTestId("one-time-secret").waitFor();
  const link = await readSecret(page);
  await shot("workspace-invite-link");
  await closeSecret(page, link);
  await page.getByRole("table", { name: /Invites for this workspace/ }).getByText("kim@example.com").waitFor();
  await page.getByRole("button", { name: "Revoke invite to kim@example.com" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Revoke invite" }).click();
  await toast(page, "Revoked the invite to kim@example.com.");
}

async function scopedReaderView(ctx, sam) {
  const { server } = ctx;
  const { page } = sam;
  await page.goto(`${server.baseUrl}/`);
  await page.getByText("Acme reports site").first().waitFor();
  check((await page.getByText("Acme private site").count()) === 0, "a folder-scoped reader does not see the other sites of the workspace");
  await page.goto(`${server.baseUrl}/workspaces/acme-ltd`);
  await page.getByRole("heading", { name: "Acme Ltd", level: 1 }).waitFor();
  check((await page.getByRole("navigation", { name: "Workspace sections" }).getByRole("link", { name: "Members", exact: true }).count()) === 0, "a scoped reader gets no Members tab");
  check((await page.getByRole("navigation", { name: "Workspace sections" }).getByRole("link", { name: "Settings", exact: true }).count()) === 0, "a scoped reader gets no Settings tab");
  await openTab(page, "Analytics");
  await page.getByText("Workspace analytics is not available to you.").waitFor();
  await ctx.shotOf(page, "scoped-reader-analytics");
  await page.goto(`${server.baseUrl}/sites/acme-private-site`);
  await page.getByRole("heading", { name: "Site not found" }).waitFor();
  await page.goto(`${server.baseUrl}/sites/acme-reports-site`);
  await page.getByRole("heading", { name: "Acme reports site", level: 1 }).waitFor();
  check((await page.getByRole("button", { name: "Save changes" }).count()) === 0, "a reader gets no save button");
  await page.goto(`${server.baseUrl}/admin`);
  await page.getByRole("heading", { name: "Page not found" }).waitFor();
  await expectNoHorizontalScroll(page, `${ctx.combo.name} scoped reader`, ctx.problems);
}

async function disableAndEnable(ctx, sam) {
  const { page, server, shot } = ctx;
  await page.goto(`${server.baseUrl}/admin/users`);
  await page.getByRole("button", { name: `Disable ${SAM.email}` }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Disable account" }).click();
  await toast(page, `Disabled ${SAM.email}.`);
  await page.getByRole("button", { name: `Enable ${SAM.email}` }).waitFor();
  await shot("admin-users");
  await sam.page.goto(`${server.baseUrl}/`);
  await sam.page.getByRole("heading", { name: "Sign in" }).waitFor();
  await page.getByRole("button", { name: `Enable ${SAM.email}` }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Enable account" }).click();
  await toast(page, `Enabled ${SAM.email}.`);
  await sam.page.getByLabel("Email").fill(SAM.email);
  await sam.page.getByLabel("Password", { exact: true }).fill(SAM.password);
  await sam.page.getByRole("button", { name: "Sign in" }).click();
  await sam.page.getByRole("heading", { name: "Sites", level: 1 }).waitFor();
  await page.getByRole("button", { name: `Create a reset link for ${SAM.email}` }).click();
  await page.getByTestId("one-time-secret").waitFor();
  const link = await readSecret(page);
  check(link.includes("/reset-password/"), "a reset link points at the reset page");
  await closeSecret(page, link);
}

export async function teamFlow(ctx) {
  await createWorkspace(ctx);
  const link = await inviteFromAdmin(ctx);
  const sam = await acceptInvite(ctx, link);
  await addScopedReader(ctx);
  await workspaceInvite(ctx);
  await scopedReaderView(ctx, sam);
  await disableAndEnable(ctx, sam);
  await sam.context.close();
  await sessionApi(ctx.page, ctx.server.baseUrl, "GET", "/workspaces");
}
