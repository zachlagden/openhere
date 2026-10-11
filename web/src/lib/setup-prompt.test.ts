import { describe, expect, it } from "vitest";
import { SKILL_INSTALL_COMMAND, SKILL_NAME, SKILL_REPOSITORY } from "@/brand";
import { buildSetupPrompt } from "./setup-prompt";

const base = { instanceUrl: "https://sites.example.com/", credentialsPath: "~/.openhere/credentials", userEmail: "alex@example.com" };

describe("buildSetupPrompt", () => {
  it("takes the install command from the brand constants", () => {
    expect(SKILL_INSTALL_COMMAND).toBe(`npx skills add ${SKILL_REPOSITORY} --skill ${SKILL_NAME} -g`);
    const prompt = buildSetupPrompt({ ...base, emailAvailable: true });
    expect(prompt).toContain(SKILL_INSTALL_COMMAND);
    expect(prompt).toContain("npx skills add zachlagden/openhere --skill openhere -g");
  });

  it("names the instance URL without a trailing slash", () => {
    const prompt = buildSetupPrompt({ ...base, emailAvailable: true });
    expect(prompt).toContain("My instance URL is https://sites.example.com.");
    expect(prompt).not.toContain("com//");
  });

  it("uses the email code flow when email is available", () => {
    const prompt = buildSetupPrompt({ ...base, emailAvailable: true });
    expect(prompt).toContain("Sign in with an email code");
    expect(prompt).toContain("alex@example.com");
    expect(prompt).toContain("~/.openhere/credentials");
    expect(prompt).not.toContain("settings/api-keys");
  });

  it("asks for an API key when email is not available", () => {
    const prompt = buildSetupPrompt({ ...base, emailAvailable: false });
    expect(prompt).toContain("https://sites.example.com/settings/api-keys");
    expect(prompt).not.toContain("email code");
    expect(prompt).not.toContain("alex@example.com");
  });

  it("gives agents without skill support the agent guide and says it needs the key", () => {
    const prompt = buildSetupPrompt({ ...base, emailAvailable: true });
    expect(prompt).toContain("https://sites.example.com/api/v1/docs/agent-guide.md (it needs the key)");
    expect(prompt).toContain("If you cannot install skills");
  });

  it("stays short and never asks the agent to print the key", () => {
    const prompt = buildSetupPrompt({ ...base, emailAvailable: true });
    expect(prompt.split("\n").length).toBeLessThanOrEqual(15);
    expect(prompt).toContain("Never print, log or commit the key");
  });
});
