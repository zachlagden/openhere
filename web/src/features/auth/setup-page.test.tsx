import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { alex } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { SetupPage } from "./setup-page";

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderSetup(): void {
  renderWithApp(
    <Routes>
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/" element={<p>Home</p>} />
      <Route path="/login" element={<p>Login page</p>} />
    </Routes>,
    "/setup",
  );
}

async function fill(token: string): Promise<void> {
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("Setup token"), token);
  await user.type(screen.getByLabelText("Email"), "alex@example.com");
  await user.type(screen.getByLabelText("Password", { exact: true }), "a long example password");
  await user.click(screen.getByRole("button", { name: "Create account" }));
}

describe("SetupPage", () => {
  it("explains where the token comes from", async () => {
    mockApi({ "GET /api/v1/setup/status": { body: { setupRequired: true } } });
    renderSetup();
    expect(await screen.findByText(/setup token from the server log/i)).toBeInTheDocument();
    expect(screen.getByText("setup_token")).toBeInTheDocument();
  });

  it("creates the root user and goes home", async () => {
    const fetchMock = mockApi({
      "GET /api/v1/setup/status": { body: { setupRequired: true } },
      "POST /api/v1/setup": { status: 201, body: { user: alex } },
    });
    renderSetup();
    await fill("  token-123 ");
    expect(await screen.findByText("Home")).toBeInTheDocument();
    const call = fetchMock.mock.calls.find(([url]) => String(url).endsWith("/api/v1/setup")) as [string, RequestInit];
    expect(JSON.parse(String(call[1].body))).toMatchObject({ setupToken: "token-123", email: "alex@example.com" });
  });

  it("shows a wrong token", async () => {
    mockApi({
      "GET /api/v1/setup/status": { body: { setupRequired: true } },
      "POST /api/v1/setup": apiError(403, "invalid_setup_token", "The setup token is not valid."),
    });
    renderSetup();
    await fill("nope");
    expect(await screen.findByRole("alert")).toHaveTextContent("The setup token is not valid.");
  });

  it("redirects to login once setup is done", async () => {
    mockApi({ "GET /api/v1/setup/status": { body: { setupRequired: false } } });
    renderSetup();
    expect(await screen.findByText("Login page")).toBeInTheDocument();
  });
});
