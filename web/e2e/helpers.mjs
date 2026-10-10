import { check } from "./checks.mjs";

export const VIEWER_AGENT = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36";

export async function sessionApi(page, baseUrl, method, route, body) {
  const response = await page.context().request.fetch(`${baseUrl}/api/v1${route}`, {
    method,
    headers: { "X-Requested-With": "e2e", Origin: baseUrl, "Content-Type": "application/json" },
    data: body,
  });
  check(response.ok(), `${method} ${route} answered ${response.status()}`);
  return response.status() === 204 ? null : response.json();
}

export async function toast(page, text) {
  try {
    await page.locator('[aria-live="polite"]').getByText(text).first().waitFor({ timeout: 8000 });
  } catch (error) {
    const alerts = await page.locator('[role="alert"], [role="status"]').allInnerTexts();
    throw new Error(`Expected the message "${text}". The page shows: ${alerts.join(" | ")}`, { cause: error });
  }
}

export async function openTab(page, name) {
  await page.getByRole("navigation", { name: /sections/i }).getByRole("link", { name, exact: true }).click();
}

export async function readSecret(page) {
  const secret = await page.getByTestId("one-time-secret").innerText();
  return secret.trim();
}

export async function closeSecret(page, secret) {
  await page.getByRole("button", { name: "I have saved it" }).click();
  await page.getByRole("dialog").waitFor({ state: "hidden" });
  const body = await page.locator("body").innerText();
  check(!body.includes(secret), "a one-time secret disappears from the page when its dialog closes");
}

export async function viewSite(baseUrl, slug, count) {
  for (let index = 0; index < count; index += 1) {
    const response = await fetch(`${baseUrl}/s/${slug}/`, { headers: { "User-Agent": VIEWER_AGENT, Accept: "text/html" } });
    check(response.ok, `the site ${slug} answers its visitors`);
    await response.text();
  }
}

export async function waitForViews(page, baseUrl, slug, minimum) {
  const deadline = Date.now() + 25_000;
  while (Date.now() < deadline) {
    const report = await sessionApi(page, baseUrl, "GET", `/sites/${slug}/analytics`);
    if (report.totals.views >= minimum) {
      return report;
    }
    await page.waitForTimeout(1000);
  }
  throw new Error(`The analytics for ${slug} never reached ${minimum} views.`);
}
