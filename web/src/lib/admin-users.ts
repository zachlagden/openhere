import { isApiError } from "@/api/client";
import { describeError } from "@/api/errors";
import type { AdminUser } from "@/api/types";

export function describeUserError(error: unknown): string {
  if (isApiError(error)) {
    switch (error.code) {
      case "cannot_disable_self":
        return "You cannot disable your own account. Ask another root user to do it.";
      case "last_root":
        return "This is the last root user, so it cannot be disabled. Nobody could manage the server without it.";
      case "user_disabled":
        return "This account is disabled. Enable it before you create a reset link.";
    }
  }
  return describeError(error).message;
}

export function canDisable(user: AdminUser, currentUserId: string | undefined): boolean {
  return !user.disabled && user.id !== currentUserId;
}

export function canResetPassword(user: AdminUser): boolean {
  return !user.disabled;
}

export function userNote(user: AdminUser, currentUserId: string | undefined): string | null {
  if (user.id === currentUserId) {
    return "This is you.";
  }
  return null;
}

export function sortUsers(users: AdminUser[]): AdminUser[] {
  return [...users].sort((left, right) => Number(right.isRoot) - Number(left.isRoot) || left.email.localeCompare(right.email));
}
