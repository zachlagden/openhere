import { describe, expect, it } from "vitest";
import { buildSetupPrompt } from "./setup-prompt";

const base = { instanceUrl: "https://sites.example.com/", credentialsPath: "~/.openhere/credentials", userEmail: "alex@example.com" };

describe("buildSetupPrompt", () => {
  it("uses the email code flow when email is available", () => {
    const prompt = buildSetupPrompt({ ...base, emailAvailable: true });
    expect(prompt).toContain("POST https://sites.example.com/api/v1/auth/agent/request-code");
    expect(prompt).toContain('{"email": "alex@example.com"}');
    expect(prompt).toContain("verify-code");
    expect(prompt).not.toContain("settings/api-keys");
  });

  it("asks for an API key when email is not available", () => {
    const prompt = buildSetupPrompt({ ...base, emailAvailable: false });
    expect(prompt).toContain("https://sites.example.com/settings/api-keys");
    expect(prompt).not.toContain("request-code");
  });

  it("names the guide, the credentials file and the publish calls", () => {
    const prompt = buildSetupPrompt({ ...base, emailAvailable: true });
    expect(prompt).toContain("GET https://sites.example.com/api/v1/docs/agent-guide.md");
    expect(prompt).toContain("~/.openhere/credentials");
    expect(prompt).toContain("POST https://sites.example.com/api/v1/publish");
    expect(prompt).not.toContain("com//api");
  });
});
