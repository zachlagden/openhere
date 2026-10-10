import { check } from "./checks.mjs";
import { toast } from "./helpers.mjs";

export async function adminFlow(ctx) {
  const { page, server, shot } = ctx;
  await page.goto(`${server.baseUrl}/admin/policy`);
  const field = page.getByLabel("Versions kept per site");
  await field.waitFor();
  const save = page.getByRole("button", { name: "Save changes" });
  check(await save.isDisabled(), "Save stays disabled until a setting changes");
  await field.fill("12");
  check(await save.isEnabled(), "Save enables once a setting changes");
  await save.click();
  await toast(page, "Settings saved.");
  await page.reload();
  check((await page.getByLabel("Versions kept per site").inputValue()) === "12", "a saved setting persists");
  await field.fill("0");
  await page.getByRole("button", { name: "Save changes" }).click();
  await page.getByText(/Setting retention.keepVersions must be between/).waitFor();
  await shot("admin-policy-error");
  await page.getByRole("button", { name: "Discard changes" }).click();
  await page.goto(`${server.baseUrl}/admin/hosting`);
  await page.getByRole("heading", { name: "Site addresses" }).waitFor();
  await page.getByRole("button", { name: "Check now" }).click();
  await page.getByText(/The check (passed|failed)/).waitFor({ timeout: 20_000 });
  await shot("admin-hosting");
}
