import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { accountFlow } from "./account-flow.mjs";
import { adminFlow } from "./admin-flow.mjs";
import { watchConsole } from "./checks.mjs";
import { fullFlow, prepareServer } from "./flow.mjs";
import { siteFlow } from "./site-flow.mjs";
import { startServer } from "./server.mjs";
import { teamFlow } from "./team-flow.mjs";
import { tour } from "./tour.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "../..");
const outDir = process.env.E2E_OUT ?? path.join(here, "output");
const chromePath = process.env.CHROME_PATH ?? "/usr/bin/google-chrome";

function loadPlaywright() {
  try {
    return createRequire(import.meta.url)("playwright-core");
  } catch (error) {
    throw new Error("playwright-core is not installed where Node can find it. Set NODE_PATH.", { cause: error });
  }
}

function buildBinary() {
  if (process.env.OPENHERE_BIN) {
    return process.env.OPENHERE_BIN;
  }
  const binary = path.join(tmpdir(), "openhere-e2e-binary");
  execFileSync("go", ["build", "-o", binary, "./cmd/openhere"], { cwd: repo, stdio: "inherit" });
  return binary;
}

const COMBOS = [
  { name: "desktop-light", viewport: { width: 1280, height: 800 }, colorScheme: "light" },
  { name: "desktop-dark", viewport: { width: 1280, height: 800 }, colorScheme: "dark" },
  { name: "phone-light", viewport: { width: 390, height: 844 }, colorScheme: "light", mobile: true },
  { name: "phone-dark", viewport: { width: 390, height: 844 }, colorScheme: "dark", mobile: true },
];

async function runCombo(browser, combo, index, problems) {
  const server = await startServer(buildBinary());
  const context = await browser.newContext({
    viewport: combo.viewport,
    colorScheme: combo.colorScheme,
    isMobile: combo.mobile ?? false,
    hasTouch: combo.mobile ?? false,
    permissions: ["clipboard-read", "clipboard-write"],
  });
  try {
    const page = await context.newPage();
    watchConsole(page, combo.name, problems);
    const shot = (name) => page.screenshot({ path: path.join(outDir, `${combo.name}-${name}.png`), fullPage: true });
    const key = index === 0 ? await fullFlow(page, server, shot) : await prepareServer(page, server);
    const flowContext = {
      page,
      server,
      shot,
      key,
      browser,
      combo,
      problems,
      watch: (other, name) => watchConsole(other, `${combo.name} ${name}`, problems),
      shotOf: (other, name) => other.screenshot({ path: path.join(outDir, `${combo.name}-${name}.png`), fullPage: true }),
    };
    await siteFlow(flowContext);
    await accountFlow(flowContext);
    await teamFlow(flowContext);
    await adminFlow(flowContext);
    await tour(page, combo.name, shot, problems);
  } finally {
    await context.close();
    await server.stop();
  }
}

async function main() {
  const { chromium } = loadPlaywright();
  mkdirSync(outDir, { recursive: true });
  const binary = buildBinary();
  process.env.OPENHERE_BIN = binary;
  const browser = await chromium.launch({ executablePath: chromePath, headless: true });
  const problems = [];
  try {
    for (const [index, combo] of COMBOS.entries()) {
      await runCombo(browser, combo, index, problems);
      console.log(`ok ${combo.name}`);
    }
  } finally {
    await browser.close();
  }
  if (problems.length > 0) {
    console.error(problems.join("\n"));
    process.exitCode = 1;
    return;
  }
  console.log(`All checks passed. Screenshots are in ${outDir}.`);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
