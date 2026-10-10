import { describe, expect, it } from "vitest";
import { makeSite } from "@/test/fixtures";
import { modeSwitchedOffBySystem, noticeMessages, noticeSeverity } from "./hosting-notices";

describe("noticeSeverity", () => {
  it("is info for a reachable site whose other mode is simply off", () => {
    expect(noticeSeverity(makeSite())).toBe("info");
  });

  it("warns when a published site is unreachable", () => {
    const site = makeSite({ hosting: { ...makeSite().hosting, reachable: false } });
    expect(noticeSeverity(site)).toBe("warning");
  });

  it("does not warn about a site nothing was published to", () => {
    const site = makeSite({ currentVersionId: null, hosting: { ...makeSite().hosting, reachable: false } });
    expect(noticeSeverity(site)).toBe("info");
  });

  it("warns when the system switched a mode off", () => {
    const base = makeSite();
    const hosting = { ...base.hosting, modes: { ...base.hosting.modes, subdomain: { state: "off" as const, reason: "wildcard_check_failed" as const } } };
    expect(modeSwitchedOffBySystem(hosting)).toBe(true);
    expect(noticeSeverity(makeSite({ hosting }))).toBe("warning");
  });
});

describe("noticeMessages", () => {
  it("hides quiet notices in the list and shows them on the detail page", () => {
    const site = makeSite();
    expect(noticeMessages(site, "list")).toEqual([]);
    expect(noticeMessages(site, "detail")).toEqual(["The subdomain address for this site is switched off."]);
  });
});
