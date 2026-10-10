const EXPECTED_STATUS = /status of (401|403|404|429)\b/;

export function watchConsole(page, label, problems) {
  page.on("console", (message) => {
    if (message.type() !== "error" && message.type() !== "warning") {
      return;
    }
    if (EXPECTED_STATUS.test(message.text())) {
      return;
    }
    problems.push(`${label}: console ${message.type()}: ${message.text()}`);
  });
  page.on("pageerror", (error) => problems.push(`${label}: page error: ${error.message}`));
}

export function check(condition, message) {
  if (!condition) {
    throw new Error(`Check failed: ${message}`);
  }
}

export async function expectNoHorizontalScroll(page, label, problems) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  if (overflow > 0) {
    problems.push(`${label}: the page scrolls horizontally by ${overflow}px`);
  }
}
