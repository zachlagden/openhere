import { act, fireEvent, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { copyText } from "@/lib/clipboard";
import { renderWithApp } from "@/test/render";
import { CopyButton, CopyLabelButton } from "./copy-button";
import { CopyField } from "./copy-field";

vi.mock("@/lib/clipboard", () => ({ copyText: vi.fn() }));

const mockedCopy = vi.mocked(copyText);

interface Clicker {
  click: (element: HTMLElement) => Promise<void>;
}

function setup(): Clicker {
  return {
    click: async (element) => {
      await act(async () => {
        fireEvent.click(element);
        await vi.advanceTimersByTimeAsync(0);
      });
    },
  };
}

function liveRegion(): HTMLElement {
  const region = document.querySelector<HTMLElement>('[aria-live="polite"]');
  if (region === null) {
    throw new Error("no live region");
  }
  return region;
}

async function flushed<T>(read: () => T): Promise<T> {
  await tick(0);
  return read();
}

async function tick(ms: number): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

describe("CopyButton", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockedCopy.mockReset();
    mockedCopy.mockResolvedValue(true);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("copies the value, confirms with a tick for 2 seconds, then reverts", async () => {
    const user = setup();
    renderWithApp(<CopyButton value="sk_example" label="Copy api key" />);
    const button = screen.getByRole("button", { name: "Copy api key" });
    expect(button.querySelector("svg.lucide-copy")).not.toBeNull();
    expect(liveRegion()).toHaveTextContent("");

    button.focus();
    await user.click(button);
    expect(mockedCopy).toHaveBeenCalledWith("sk_example");
    const confirmed = await flushed(() => screen.getByRole("button", { name: "Copied" }));
    expect(confirmed.querySelector("svg.lucide-check")).not.toBeNull();
    expect(confirmed.querySelector("svg.lucide-copy")).toBeNull();
    expect(liveRegion()).toHaveTextContent("Copied");
    
    await tick(1999);
    expect(screen.getByRole("button", { name: "Copied" })).toBeInTheDocument();
    await tick(1);
    const reverted = screen.getByRole("button", { name: "Copy api key" });
    expect(reverted.querySelector("svg.lucide-copy")).not.toBeNull();
    expect(liveRegion()).toHaveTextContent("");
  });

  it("restarts the timer on a repeated click", async () => {
    const user = setup();
    renderWithApp(<CopyButton value="sk_example" label="Copy api key" />);
    await user.click(screen.getByRole("button", { name: "Copy api key" }));
    await flushed(() => screen.getByRole("button", { name: "Copied" }));
    await tick(1500);
    await user.click(screen.getByRole("button", { name: "Copied" }));
    await tick(1500);
    expect(screen.getByRole("button", { name: "Copied" })).toBeInTheDocument();
    await tick(500);
    expect(screen.getByRole("button", { name: "Copy api key" })).toBeInTheDocument();
    expect(mockedCopy).toHaveBeenCalledTimes(2);
  });

  it("shows a failure state, no tick, and calls onFailure", async () => {
    mockedCopy.mockResolvedValue(false);
    const onFailure = vi.fn();
    const user = setup();
    renderWithApp(<CopyButton value="sk_example" label="Copy api key" onFailure={onFailure} />);
    await user.click(screen.getByRole("button", { name: "Copy api key" }));
    const failed = await flushed(() => screen.getByRole("button", { name: "Copy failed" }));
    expect(failed.querySelector("svg.lucide-x")).not.toBeNull();
    expect(failed.querySelector("svg.lucide-check")).toBeNull();
    expect(liveRegion()).toHaveTextContent("Copy failed");
    expect(onFailure).toHaveBeenCalledTimes(1);
    await tick(2000);
    expect(screen.getByRole("button", { name: "Copy api key" })).toBeInTheDocument();
  });

  it("shows no toast on success or failure", async () => {
    const user = setup();
    renderWithApp(<CopyButton value="sk_example" label="Copy api key" />);
    await user.click(screen.getByRole("button", { name: "Copy api key" }));
    await flushed(() => screen.getByRole("button", { name: "Copied" }));
    expect(screen.queryByText(/copied\./i)).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("clears the timer on unmount", async () => {
    const user = setup();
    const view = renderWithApp(<CopyButton value="sk_example" label="Copy api key" />);
    await user.click(screen.getByRole("button", { name: "Copy api key" }));
    await flushed(() => screen.getByRole("button", { name: "Copied" }));
    expect(vi.getTimerCount()).toBeGreaterThan(0);
    view.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("CopyLabelButton", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renames itself to Copied and Copy failed", () => {
    const { rerender } = renderWithApp(<CopyLabelButton label="Copy agent setup prompt" status="idle" onCopy={() => undefined} />);
    expect(screen.getByRole("button", { name: "Copy agent setup prompt" })).toBeInTheDocument();
    rerender(<CopyLabelButton label="Copy agent setup prompt" status="copied" onCopy={() => undefined} />);
    expect(screen.getByRole("button", { name: "Copied" }).querySelector("svg.lucide-check")).not.toBeNull();
    expect(liveRegion()).toHaveTextContent("Copied");
  });
});

describe("CopyField", () => {
  beforeEach(() => {
    mockedCopy.mockReset();
  });

  it("keeps the value visible and selects it when the copy fails", async () => {
    mockedCopy.mockResolvedValue(false);
    renderWithApp(<CopyField label="API key" value="sk_example_key" testId="value" />);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Copy api key" }));
    });
    await screen.findByRole("button", { name: "Copy failed" });
    expect(screen.getByTestId("value")).toHaveTextContent("sk_example_key");
    expect(window.getSelection()?.toString()).toBe("sk_example_key");
  });
});
