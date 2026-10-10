import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { pageTitle, usePageTitle } from "./use-page-title";

describe("pageTitle", () => {
  it("joins named parts before the product name and skips empty ones", () => {
    expect(pageTitle(["Launch page", undefined, "Access"])).toBe("Launch page | Access | selfhost");
    expect(pageTitle([])).toBe("selfhost");
  });
});

describe("usePageTitle", () => {
  it("sets the document title", () => {
    renderHook(() => usePageTitle("Settings"));
    expect(document.title).toBe("Settings | selfhost");
  });
});
