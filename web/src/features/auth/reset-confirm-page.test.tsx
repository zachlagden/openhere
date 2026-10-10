import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { ResetConfirmPage } from "./reset-confirm-page";

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderPage(): void {
  renderWithApp(
    <Routes>
      <Route path="/reset-password/:token" element={<ResetConfirmPage />} />
    </Routes>,
    "/reset-password/example-token",
  );
}

async function fill(): Promise<void> {
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("New password"), "a long example password");
  await user.type(screen.getByLabelText("Repeat the new password"), "a long example password");
}

describe("ResetConfirmPage", () => {
  it("keeps API keys by default", async () => {
    const fetchMock = mockApi({ "POST /api/v1/auth/password-reset/confirm": { status: 204 } });
    renderPage();
    await fill();
    await userEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("Password changed")).toBeInTheDocument();
    const call = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(call[1].body))).toEqual({ token: "example-token", password: "a long example password", revokeApiKeys: false });
  });

  it("revokes API keys when asked", async () => {
    const fetchMock = mockApi({ "POST /api/v1/auth/password-reset/confirm": { status: 204 } });
    renderPage();
    await fill();
    await userEvent.click(screen.getByRole("switch", { name: "Also revoke my API keys" }));
    await userEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("Password changed")).toBeInTheDocument();
    const call = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(call[1].body))).toMatchObject({ revokeApiKeys: true });
  });

  it("tells the user when the server is busy", async () => {
    mockApi({ "POST /api/v1/auth/password-reset/confirm": apiError(503, "server_busy", "The server is busy. Try again in a moment.", { retry_after: 5 }) });
    renderPage();
    await fill();
    await userEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("The server is busy. Try again in 5 seconds.");
  });
});
