import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useUnsavedWarning } from "./use-unsaved-warning";

function leave(): boolean {
  const event = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(event);
  return event.defaultPrevented;
}

describe("useUnsavedWarning", () => {
  it("asks the browser to confirm leaving only while the form is dirty", () => {
    const { rerender, unmount } = renderHook(({ dirty }) => useUnsavedWarning(dirty), { initialProps: { dirty: false } });
    expect(leave()).toBe(false);
    rerender({ dirty: true });
    expect(leave()).toBe(true);
    rerender({ dirty: false });
    expect(leave()).toBe(false);
    rerender({ dirty: true });
    unmount();
    expect(leave()).toBe(false);
  });
});
