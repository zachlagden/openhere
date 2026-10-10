import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SearchInput, SEARCH_DELAY_MS } from "./search-input";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("SearchInput", () => {
  it("debounces typing into one search", () => {
    const onSearch = vi.fn();
    render(<SearchInput value="" onSearch={onSearch} />);
    const box = screen.getByRole("searchbox");
    fireEvent.change(box, { target: { value: "l" } });
    fireEvent.change(box, { target: { value: "la" } });
    fireEvent.change(box, { target: { value: "launch" } });
    act(() => {
      vi.advanceTimersByTime(SEARCH_DELAY_MS - 1);
    });
    expect(onSearch).not.toHaveBeenCalled();
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onSearch).toHaveBeenCalledOnce();
    expect(onSearch).toHaveBeenCalledWith("launch");
  });

  it("searches at once on Enter", () => {
    const onSearch = vi.fn();
    render(<SearchInput value="" onSearch={onSearch} />);
    const box = screen.getByRole("searchbox");
    fireEvent.change(box, { target: { value: "docs" } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSearch).toHaveBeenCalledWith("docs");
    act(() => {
      vi.advanceTimersByTime(SEARCH_DELAY_MS * 2);
    });
    expect(onSearch).toHaveBeenCalledOnce();
  });

  it("clears with the button", () => {
    const onSearch = vi.fn();
    render(<SearchInput value="abc" onSearch={onSearch} />);
    fireEvent.click(screen.getByRole("button", { name: "Clear search" }));
    expect(onSearch).toHaveBeenCalledWith("");
    expect(screen.getByRole("searchbox")).toHaveValue("");
  });

  it("focuses with the slash key", () => {
    render(<SearchInput value="" onSearch={() => undefined} />);
    fireEvent.keyDown(document.body, { key: "/" });
    expect(screen.getByRole("searchbox")).toHaveFocus();
  });
});
