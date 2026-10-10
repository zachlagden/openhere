import { describe, expect, it } from "vitest";
import type { Access } from "@/api/types";
import { accessProblem, buildAccessPatch, draftFromAccess } from "./access-form";

const access: Access = { mode: "public", hasPassword: false, emailAllowlist: ["sam@example.com"], domainAllowlist: [], emailCodeAvailable: true };

describe("buildAccessPatch", () => {
  it("is empty for an untouched draft", () => {
    expect(buildAccessPatch(access, draftFromAccess(access))).toEqual({});
  });

  it("sends only the changed fields", () => {
    const draft = { ...draftFromAccess(access), mode: "restricted" as const, domainAllowlist: ["example.com"] };
    expect(buildAccessPatch(access, draft)).toEqual({ mode: "restricted", domainAllowlist: ["example.com"] });
  });

  it("sends a typed password and nothing for an empty one", () => {
    expect(buildAccessPatch(access, { ...draftFromAccess(access), password: "long enough pw" })).toEqual({ password: "long enough pw" });
    expect(buildAccessPatch(access, { ...draftFromAccess(access), password: "" })).toEqual({});
  });
});

describe("accessProblem", () => {
  it("asks for a password before password mode", () => {
    expect(accessProblem(access, { ...draftFromAccess(access), mode: "password" })).toMatch(/Set a password/);
  });

  it("accepts password mode when a password exists", () => {
    expect(accessProblem({ ...access, hasPassword: true }, { ...draftFromAccess(access), mode: "password" })).toBeNull();
  });

  it("rejects a short password", () => {
    expect(accessProblem(access, { ...draftFromAccess(access), password: "short" })).toMatch(/8 to 128/);
  });
});
