import { check } from "./checks.mjs";
import { publishSite } from "./publish.mjs";

export const CREDENTIALS = { email: "alex@example.com", password: "a long example password" };

export async function signIn(page, baseUrl) {
  await page.goto(`${baseUrl}/login`);
  await page.getByLabel("Email").fill(CREDENTIALS.email);
  await page.getByLabel("Password", { exact: true }).fill(CREDENTIALS.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("heading", { name: "Sites", level: 1 }).waitFor();
}

export async function signOut(page) {
  await page.getByRole("button", { name: "Account menu" }).click();
  await page.getByRole("menuitem", { name: "Sign out" }).click();
  await page.getByRole("heading", { name: "Sign in" }).waitFor();
}

async function completeSetup(page, server, shot) {
  await page.goto(`${server.baseUrl}/`);
  await page.getByRole("heading", { name: "Set up your server" }).waitFor();
  check(new URL(page.url()).pathname === "/setup", "an unconfigured server sends visitors to /setup");
  await shot("setup");
  await page.getByLabel("Setup token").fill("wrong-token");
  await page.getByLabel("Email").fill(CREDENTIALS.email);
  await page.getByLabel("Password", { exact: true }).fill(CREDENTIALS.password);
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByText("The setup token is not valid.").waitFor();
  await shot("setup-wrong-token");
  await page.getByLabel("Setup token").fill(server.setupToken);
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByRole("heading", { name: "Sites", level: 1 }).waitFor();
  await page.getByRole("heading", { name: "No sites yet" }).waitFor();
  await shot("sites-empty");
}

async function createFolder(page, shot) {
  await page.getByRole("link", { name: /Personal/ }).first().click();
  await page.getByRole("button", { name: "New folder" }).click();
  await page.getByRole("dialog").getByLabel("Name").fill("Reports");
  await shot("folder-dialog");
  await page.getByRole("button", { name: "Create folder" }).click();
  await page.getByRole("link", { name: "Reports" }).waitFor();
}

async function createApiKey(page, baseUrl) {
  const response = await page.context().request.post(`${baseUrl}/api/v1/me/keys`, {
    headers: { "X-Requested-With": "e2e", Origin: baseUrl },
    data: { label: "e2e" },
  });
  check(response.ok(), "creating an API key with the session cookie works");
  return (await response.json()).key;
}

async function searchAndMove(page, shot) {
  await page.getByRole("link", { name: "All sites" }).click();
  await page.getByText("Launch page").first().waitFor();
  await shot("sites-list");

  const search = page.getByRole("searchbox", { name: /Search sites/ });
  await search.fill("launch");
  await page.waitForURL(/[?&]q=launch/);
  await page.getByText("Launch page").first().waitFor();
  await search.fill("zzz-no-match");
  await page.getByRole("heading", { name: "No sites match" }).waitFor();
  await shot("sites-no-match");
  await page.getByRole("button", { name: "Clear search and filters" }).click();
  await page.getByText("Launch page").first().waitFor();

  await page.getByRole("button", { name: "Actions for Launch page" }).click();
  await page.getByRole("menuitem", { name: "Move to folder" }).click();
  await page.getByRole("dialog").getByLabel("Folder").selectOption({ label: "Reports" });
  await shot("move-dialog");
  await page.getByRole("button", { name: "Move site" }).click();
  await page.getByText("Moved Launch page.").waitFor();

  await page.getByRole("link", { name: "Reports" }).click();
  await page.getByText("Launch page").first().waitFor();
  await page.getByRole("link", { name: "Unfiled" }).click();
  await page.getByRole("heading", { name: "No sites match" }).waitFor();
  await page.getByRole("link", { name: "Reports" }).click();
  await page.getByText("Launch page").first().waitFor();
  await shot("sites-in-folder");
}

async function copyPrompt(page, baseUrl) {
  await page.getByRole("button", { name: "Copy agent setup prompt" }).click();
  await page.getByText("Agent setup prompt copied.").waitFor();
  const prompt = await page.evaluate(() => navigator.clipboard.readText());
  check(prompt.includes(`${baseUrl}/api/v1/docs/agent-guide.md`), "the prompt names the agent guide URL");
  check(prompt.includes(`${baseUrl}/settings/api-keys`), "without email the prompt points to the API keys settings");
  check(!prompt.includes("request-code"), "without email the prompt skips the email code flow");
}

export async function fullFlow(page, server, shot) {
  await completeSetup(page, server, shot);
  await createFolder(page, shot);
  const key = await createApiKey(page, server.baseUrl);
  await publishSite(server.baseUrl, key, {
    slug: "launch-page",
    displayName: "Launch page",
    files: { "index.html": "<!doctype html><title>Launch</title><h1>Launch</h1>" },
  });
  await page.reload();
  await searchAndMove(page, shot);
  await copyPrompt(page, server.baseUrl);
  await signOut(page);
  await shot("login");
  await signIn(page, server.baseUrl);
}
