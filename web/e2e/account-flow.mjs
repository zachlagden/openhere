import { check } from "./checks.mjs";
import { closeSecret, readSecret, toast } from "./helpers.mjs";

export async function accountFlow(ctx) {
  const { page, server, shot } = ctx;
  await page.goto(`${server.baseUrl}/settings/api-keys`);
  await page.getByRole("heading", { name: "API keys", level: 2 }).waitFor();
  await page.getByRole("button", { name: "Create key" }).click();
  await page.getByRole("dialog").getByLabel("Label").fill("ui key");
  await page.getByRole("dialog").getByRole("button", { name: "Create key" }).click();
  await page.getByTestId("one-time-secret").waitFor();
  const secret = await readSecret(page);
  check(secret.length > 20, "the new key is shown in full");
  await shot("settings-api-key-created");
  await page.getByRole("button", { name: "Copy api key" }).click();
  await toast(page, "API key copied.");
  const copied = await page.evaluate(() => navigator.clipboard.readText());
  check(copied === secret, "the copy button copies the full key");
  await closeSecret(page, secret);
  await page.getByText("ui key").first().waitFor();
  await page.getByRole("button", { name: "Revoke key ui key" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Revoke key" }).click();
  await toast(page, "Revoked ui key.");
  await page.getByText("Revoked").first().waitFor();
  await shot("settings-api-keys");
}
