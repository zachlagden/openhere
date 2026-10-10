import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithApp } from "@/test/render";
import { TypedConfirmDialog } from "./typed-confirm-dialog";

describe("TypedConfirmDialog", () => {
  it("enables the confirm button only after the phrase is typed", async () => {
    const onConfirm = vi.fn();
    renderWithApp(
      <TypedConfirmDialog open onOpenChange={() => undefined} title="Delete site?" phrase="launch-page" confirmLabel="Delete site" pending={false} error={null} onConfirm={onConfirm}>
        <p>This cannot be undone.</p>
      </TypedConfirmDialog>,
    );
    const button = await screen.findByRole("button", { name: "Delete site" });
    expect(button).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Type launch-page to confirm"), "launch-pag");
    expect(button).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Type launch-page to confirm"), "e");
    expect(button).toBeEnabled();
    await userEvent.click(button);
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});
