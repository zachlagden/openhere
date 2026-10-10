import { expectNoHorizontalScroll, check } from "./checks.mjs";
import { closeSecret, openTab, readSecret, sessionApi, toast, viewSite, waitForViews } from "./helpers.mjs";
import { republishSite } from "./publish.mjs";

const SITE_PASSWORD = "an example site password";

async function overview(ctx) {
  const { page, server, shot } = ctx;
  await page.goto(`${server.baseUrl}/`);
  await page.getByText("Launch page").first().waitFor();
  check((await page.getByText("subdomain address for this site is switched off").count()) === 0, "a reachable site shows no availability warning in the list");
  await page.getByRole("link", { name: "Launch page" }).first().click();
  await page.getByRole("heading", { name: "Launch page", level: 1 }).waitFor();
  await page.getByText("The subdomain address for this site is switched off.").waitFor();
  check(new URL(page.url()).pathname === "/sites/launch-page", "a site name opens its detail page");
  await shot("site-overview");
  await page.getByLabel("Display name").fill("Launch page (live)");
  await page.getByLabel("Description").fill("The public landing page.");
  await page.getByRole("button", { name: "Save changes" }).click();
  await toast(page, "Site details saved.");
  await page.getByRole("heading", { name: "Launch page (live)", level: 1 }).waitFor();
  await page.getByRole("switch", { name: "Single-page app mode" }).click();
  await toast(page, "Single-page app mode is on.");
  check(await page.getByRole("switch", { name: "Single-page app mode" }).isChecked(), "the single-page switch stays on");
}

async function access(ctx) {
  const { page, server, shot } = ctx;
  await openTab(page, "Access");
  await page.getByRole("radio", { name: /Password/ }).click();
  await page.getByText("Set a password before switching this site to password mode.").waitFor();
  await page.getByLabel("Password", { exact: true }).fill(SITE_PASSWORD);
  await shot("site-access-password");
  await page.getByRole("button", { name: "Save changes" }).click();
  await toast(page, "Access settings saved.");
  let stored = await sessionApi(page, server.baseUrl, "GET", "/publish/launch-page/access");
  check(stored.access.mode === "password" && stored.access.hasPassword, "the password mode and password are stored");
  await page.getByText("A password is set").waitFor();
  check(!(await page.locator("body").innerHTML()).includes(SITE_PASSWORD), "the password is never rendered after saving");
  await page.getByRole("radio", { name: /Public/ }).click();
  await page.getByRole("button", { name: "Save changes" }).click();
  await toast(page, "Access settings saved.");
  await page.getByRole("button", { name: "Remove password" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Remove password" }).click();
  await toast(page, "Password removed.");
  stored = await sessionApi(page, server.baseUrl, "GET", "/publish/launch-page/access");
  check(stored.access.mode === "public" && !stored.access.hasPassword, "the password is cleared");
  await page.getByRole("radio", { name: /Restricted/ }).click();
  await page.getByLabel("Add an email address").fill("sam@example.com");
  await page.getByRole("button", { name: "Add", exact: true }).first().click();
  await page.getByRole("button", { name: "Remove sam@example.com" }).waitFor();
  await page.getByText("This server cannot send email yet").waitFor();
  await shot("site-access-restricted");
  await page.getByRole("radio", { name: /Public/ }).click();
  await page.getByRole("button", { name: "Discard changes" }).click();
}

async function domains(ctx) {
  const { page, shot } = ctx;
  await openTab(page, "Domains");
  await page.getByLabel("Domain name").fill("docs.example.org");
  await page.getByRole("button", { name: "Add domain" }).click();
  const card = page.getByTestId("domain-card");
  await card.waitFor();
  await card.getByText("_selfhost-challenge.docs.example.org").waitFor();
  await card.getByRole("button", { name: "Copy TXT record value" }).waitFor();
  await card.getByText("Waiting for DNS").waitFor();
  await shot("site-domains");
  await card.getByRole("button", { name: "Check DNS now" }).click();
  await card.locator("[role=status]").first().waitFor({ timeout: 15_000 });
  await shot("site-domains-checked");
}

async function analytics(ctx) {
  const { page, server, shot, problems, combo } = ctx;
  await viewSite(server.baseUrl, "launch-page", 6);
  await waitForViews(page, server.baseUrl, "launch-page", 6);
  await openTab(page, "Analytics");
  await page.getByTestId("total-views").waitFor();
  const views = Number(await page.getByTestId("total-views").innerText());
  check(views >= 6, `the analytics tab shows the recorded views (saw ${views})`);
  await page.getByRole("img", { name: /points from/ }).waitFor();
  const chart = page.getByRole("group", { name: /left and right arrow keys/ });
  await chart.focus();
  await page.keyboard.press("End");
  await page.getByTestId("chart-tooltip").waitFor();
  await shot("site-analytics");
  await page.getByRole("button", { name: "Show data table" }).click();
  await page.getByRole("table", { name: /Views and unique visitors/ }).waitFor();
  await page.getByRole("button", { name: "Today" }).click();
  await page.waitForURL(/range=today/);
  await page.getByRole("img", { name: /points from/ }).waitFor();
  await shot("site-analytics-hourly");
  await expectNoHorizontalScroll(page, `${combo} site analytics`, problems);
}

async function versions(ctx) {
  const { page, server, shot, key } = ctx;
  await republishSite(server.baseUrl, key, "launch-page", { "index.html": "<!doctype html><title>Launch</title><h1>Launch two</h1>" });
  await openTab(page, "Versions");
  const table = page.getByRole("table", { name: /Versions of this site/ });
  await table.waitFor();
  await page.getByText("Files in the live version").waitFor();
  const rowCount = await table.getByRole("row").count();
  check(rowCount === 3, `the version history lists two versions and a header row (saw ${rowCount})`);
  await shot("site-versions");
  await page.getByRole("button", { name: /Restore version/ }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Restore version" }).click();
  await toast(page, "is live again.");
  await page.getByRole("button", { name: /Delete version/ }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete version" }).click();
  await toast(page, "Deleted version");
  check((await table.getByRole("row").count()) === 2, "a deleted version leaves the history");
}

export async function siteFlow(ctx) {
  await overview(ctx);
  await access(ctx);
  await domains(ctx);
  await analytics(ctx);
  await versions(ctx);
  const { page, server } = ctx;
  await page.goto(`${server.baseUrl}/sites/does-not-exist`);
  await page.getByRole("heading", { name: "Site not found" }).waitFor();
}

export { readSecret, closeSecret };
