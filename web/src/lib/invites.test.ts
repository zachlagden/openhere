import { describe, expect, it } from "vitest";
import { ApiError } from "@/api/client";
import type { CreatedInvite, Invite } from "@/api/types";
import { canResend, canRevoke, describeEmailOutcome, describeInviteError, inviteStatus, invitesForWorkspace } from "./invites";

function invite(overrides: Partial<Invite> = {}): Invite {
  return {
    id: "i1",
    email: "kim@example.com",
    status: "pending",
    workspace: { id: "w2", slug: "acme", name: "Acme Ltd" },
    role: "read",
    invitedBy: "alex@example.com",
    createdAt: "2026-03-01T10:00:00Z",
    expiresAt: "2026-03-08T10:00:00Z",
    acceptedAt: null,
    revokedAt: null,
    ...overrides,
  };
}

function created(overrides: Partial<CreatedInvite> = {}): CreatedInvite {
  return { invite: invite(), link: "https://sites.example.com/invite/abc", emailAvailable: true, emailSent: false, emailError: null, ...overrides };
}

describe("invite rules", () => {
  it("labels each status", () => {
    expect(inviteStatus(invite()).label).toBe("Waiting");
    expect(inviteStatus(invite({ status: "accepted" })).tone).toBe("ok");
    expect(inviteStatus(invite({ status: "expired" })).label).toBe("Expired");
    expect(inviteStatus(invite({ status: "revoked" })).label).toBe("Revoked");
  });

  it("allows revoke only while pending and resend until accepted", () => {
    expect(canRevoke(invite())).toBe(true);
    expect(canRevoke(invite({ status: "expired" }))).toBe(false);
    expect(canResend(invite({ status: "expired" }))).toBe(true);
    expect(canResend(invite({ status: "accepted" }))).toBe(false);
  });

  it("filters the list to one workspace", () => {
    const list = [invite(), invite({ id: "i2", workspace: null }), invite({ id: "i3", workspace: { id: "w9", slug: "x", name: "X" } })];
    expect(invitesForWorkspace(list, "w2").map((item) => item.id)).toEqual(["i1"]);
  });
});

describe("messages", () => {
  it("explains known errors", () => {
    expect(describeInviteError(new ApiError(409, "email_registered", "x", null))).toMatch(/Members tab/);
    expect(describeInviteError(new ApiError(403, "forbidden", "x", null))).toMatch(/not allowed/);
    expect(describeInviteError(new ApiError(400, "validation_failed", "Enter a valid email address.", null))).toBe("Enter a valid email address.");
  });

  it("reports what happened to the email", () => {
    expect(describeEmailOutcome(created({ emailSent: true }), true)).toMatch(/also emailed to kim@example.com/);
    expect(describeEmailOutcome(created({ emailError: "the server rejected the password" }), true)).toMatch(/could not be sent/);
    expect(describeEmailOutcome(created({ emailAvailable: false }), false)).toMatch(/cannot send email/);
    expect(describeEmailOutcome(created(), false)).toBeNull();
  });
});
