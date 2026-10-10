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
  await visit("/does-not-exist", () => page.getByRole("heading", { name: "Page not found" }).waitFor(), "tour-not-found");
  await visit("/invite/not-a-real-token", () => page.getByRole("heading", { name: "This invite is not valid" }).waitFor(), "tour-invite-invalid");
}

async function personalSlug(page) {
  const response = await page.context().request.get(new URL("/api/v1/workspaces", page.url()).toString());
  const { workspaces } = await response.json();
  return workspaces.find((workspace) => workspace.kind === "personal").slug;
}
