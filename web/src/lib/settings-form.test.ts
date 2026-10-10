import { describe, expect, it } from "vitest";
import { ApiError } from "@/api/client";
import type { Settings } from "@/api/types";
import { buildClearPatch, buildSettingsPatch, describeSettingsFailure, destructiveChanges, fieldError, fieldFromMessage, formError, pickSettings } from "./settings-form";

const settings = {
  "hosting.baseDomain": "example.com",
  "hosting.subdomainEnabled": true,
  "hosting.pathEnabled": true,
  "hosting.customDomainsEnabled": true,
  "hosting.maxDomainsPerSite": 10,
  "email.provider": "smtp",
  "email.smtp.host": "mail.example.com",
  "email.smtp.passwordSet": true,
  "analytics.retentionDays": 395,
} as Settings;

describe("pickSettings and buildSettingsPatch", () => {
  it("picks only the requested keys", () => {
    expect(pickSettings(settings, ["hosting.baseDomain", "hosting.maxDomainsPerSite"])).toEqual({ "hosting.baseDomain": "example.com", "hosting.maxDomainsPerSite": 10 });
  });

  it("is empty for an untouched draft", () => {
    const original = pickSettings(settings, ["hosting.baseDomain", "hosting.pathEnabled"]);
    expect(buildSettingsPatch(original, { ...original })).toEqual({});
  });

  it("sends only the changed fields", () => {
    const original = pickSettings(settings, ["hosting.baseDomain", "hosting.pathEnabled", "hosting.maxDomainsPerSite"]);
    expect(buildSettingsPatch(original, { ...original, "hosting.maxDomainsPerSite": 5 })).toEqual({ "hosting.maxDomainsPerSite": 5 });
  });

  it("includes a secret only when one was typed", () => {
    const original = pickSettings(settings, ["email.smtp.host"]);
    expect(buildSettingsPatch(original, { ...original }, { "email.smtp.password": "" })).toEqual({});
    expect(buildSettingsPatch(original, { ...original }, { "email.smtp.password": "a-new-password" })).toEqual({ "email.smtp.password": "a-new-password" });
  });

  it("drops numbers that are not finite", () => {
    const original = pickSettings(settings, ["hosting.maxDomainsPerSite"]);
    expect(buildSettingsPatch(original, { "hosting.maxDomainsPerSite": Number.NaN })).toEqual({});
  });
});

describe("error mapping", () => {
  const keys = ["email.smtp.port", "email.smtp.host", "email.provider"];

  it("finds the field named in a message, preferring the longest key", () => {
    expect(fieldFromMessage("Setting email.smtp.port must be between 1 and 65535.", keys)).toBe("email.smtp.port");
    expect(fieldFromMessage("Nothing about a key here.", keys)).toBeNull();
  });

  it("maps validation errors to a field", () => {
    const failure = describeSettingsFailure(new ApiError(400, "validation_failed", "Setting email.smtp.port must be between 1 and 65535.", null), keys);
    expect(fieldError(failure, "email.smtp.port")).toBe("Setting email.smtp.port must be between 1 and 65535.");
    expect(formError(failure)).toBeNull();
  });

  it("explains the hosting rule errors in plain language", () => {
    expect(describeSettingsFailure(new ApiError(409, "no_hosting_mode", "x", null), keys).message).toMatch(/must stay on/);
    expect(describeSettingsFailure(new ApiError(409, "wildcard_not_verified", "x", null), keys).message).toMatch(/Run the check first/);
    expect(describeSettingsFailure(new ApiError(409, "base_domain_in_use", "x", null), keys).message).toMatch(/Delete that domain first/);
  });

  it("falls back to the form for unmapped messages", () => {
    const failure = describeSettingsFailure(new ApiError(400, "validation_failed", "Send at least one setting to change.", null), keys);
    expect(formError(failure)).toBe("Send at least one setting to change.");
  });
});

describe("destructiveChanges", () => {
  it("lists what a change would break", () => {
    const original = pickSettings(settings, ["hosting.baseDomain", "hosting.subdomainEnabled", "hosting.pathEnabled", "hosting.customDomainsEnabled", "email.provider", "analytics.retentionDays"]);
    expect(destructiveChanges(original, { "hosting.subdomainEnabled": false })).toHaveLength(1);
    expect(destructiveChanges(original, { "hosting.baseDomain": "other.example" })[0]).toMatch(/resets the wildcard check/);
    expect(destructiveChanges(original, { "hosting.customDomainsEnabled": false })[0]).toMatch(/Every custom domain/);
    expect(destructiveChanges(original, { "email.provider": "none" })[0]).toMatch(/Invite emails/);
    expect(destructiveChanges(original, { "analytics.retentionDays": 90 })[0]).toMatch(/deleted/);
  });

  it("is empty for safe changes", () => {
    const original = pickSettings(settings, ["hosting.subdomainEnabled", "analytics.retentionDays"]);
    expect(destructiveChanges(original, { "hosting.subdomainEnabled": true })).toEqual([]);
    expect(destructiveChanges(original, { "analytics.retentionDays": 500 })).toEqual([]);
  });
});

describe("buildClearPatch", () => {
  it("sends an empty string for the key", () => {
    expect(buildClearPatch("tls.cloudflareApiToken")).toEqual({ "tls.cloudflareApiToken": "" });
  });
});
