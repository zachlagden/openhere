import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { alex } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { LoginPage } from "./login-page";

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderLogin(route = "/login"): void {
  renderWithApp(
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<p>Home</p>} />
      <Route path="/setup" element={<p>Setup page</p>} />
      <Route path="/docs" element={<p>Docs page</p>} />
    </Routes>,
    route,
  );
}

const signedOut = {
  "GET /api/v1/setup/status": { body: { setupRequired: false } },
  "GET /api/v1/me": apiError(401, "unauthenticated", "Authentication required."),
};

async function fillAndSubmit(): Promise<void> {
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("Email"), "alex@example.com");
  await user.type(screen.getByLabelText("Password", { exact: true }), "a long example password");
  await user.click(screen.getByRole("button", { name: "Sign in" }));
}

describe("LoginPage", () => {
  it("links the open source note to the repository", async () => {
    mockApi(signedOut);
    renderLogin();
    const link = await screen.findByRole("link", { name: "OpenHere is open source and runs on your own server." });
    expect(link).toHaveAttribute("href", "https://github.com/zachlagden/openhere");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("signs in and follows the next parameter", async () => {
    const fetchMock = mockApi({ ...signedOut, "POST /api/v1/auth/login": { body: { user: alex } } });
    renderLogin("/login?next=%2Fdocs");
    await fillAndSubmit();
    expect(await screen.findByText("Docs page")).toBeInTheDocument();
    const call = fetchMock.mock.calls.find(([url]) => String(url).endsWith("/auth/login")) as [string, RequestInit];
    expect(JSON.parse(String(call[1].body))).toEqual({ email: "alex@example.com", password: "a long example password" });
  });

  it("ignores an unsafe next parameter", async () => {
    mockApi({ ...signedOut, "POST /api/v1/auth/login": { body: { user: alex } } });
    renderLogin("/login?next=https%3A%2F%2Fevil.example.com");
    await fillAndSubmit();
    expect(await screen.findByText("Home")).toBeInTheDocument();
  });

  it("shows the generic failure message", async () => {
    mockApi({ ...signedOut, "POST /api/v1/auth/login": apiError(401, "invalid_credentials", "Incorrect email or password.") });
    renderLogin();
    await fillAndSubmit();
    expect(await screen.findByRole("alert")).toHaveTextContent("Incorrect email or password.");
  });

  it("shows the wait on a rate limit and disables the button", async () => {
    mockApi({ ...signedOut, "POST /api/v1/auth/login": apiError(429, "rate_limited", "Too many attempts.", { retry_after: 30 }) });
    renderLogin();
    await fillAndSubmit();
    expect(await screen.findByRole("alert")).toHaveTextContent("Try again in 30 seconds");
    expect(screen.getByRole("button", { name: "Sign in" })).toBeDisabled();
  });

  it("sends a visitor to setup when the server needs it", async () => {
    mockApi({ ...signedOut, "GET /api/v1/setup/status": { body: { setupRequired: true } } });
    renderLogin();
    await waitFor(() => expect(screen.getByText("Setup page")).toBeInTheDocument());
  });

  it("says so when the server is busy hashing passwords", async () => {
    mockApi({ ...signedOut, "POST /api/v1/auth/login": apiError(503, "server_busy", "The server is busy. Try again in a moment.", { retry_after: 5 }) });
    renderLogin();
    await fillAndSubmit();
    expect(await screen.findByRole("alert")).toHaveTextContent("The server is busy. Try again in 5 seconds.");
  });
});
