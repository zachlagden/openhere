import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type JSX } from "react";
import { describe, expect, it } from "vitest";
import { renderWithApp } from "@/test/render";
import { SecretDialog } from "./secret-dialog";

function Harness(): JSX.Element {
  const [secret, setSecret] = useState<string | null>("sk_example_key_value");
  return <SecretDialog title="Your new key" fieldLabel="API key" secret={secret} warning="Copy it now." onClose={() => setSecret(null)} />;
}

describe("SecretDialog", () => {
  it("shows the secret once and removes it from the page on close", async () => {
    renderWithApp(<Harness />);
    expect(await screen.findByTestId("one-time-secret")).toHaveTextContent("sk_example_key_value");
    await userEvent.click(screen.getByRole("button", { name: "I have saved it" }));
    expect(screen.queryByText("sk_example_key_value")).not.toBeInTheDocument();
    expect(document.body.textContent).not.toContain("sk_example_key_value");
  });
});
