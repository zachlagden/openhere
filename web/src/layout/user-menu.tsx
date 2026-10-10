import { BookOpen, LogOut, Monitor, Moon, Settings, Sun, User as UserIcon } from "lucide-react";
import type { JSX } from "react";
import { Link, useNavigate } from "react-router";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useToast } from "@/components/ui/toast";
import { describeError } from "@/api/errors";
import { useTheme, type ThemePreference } from "@/lib/theme";
import { useLogout, useMe } from "@/queries/session";

function isPreference(value: string): value is ThemePreference {
  return value === "system" || value === "light" || value === "dark";
}

export function UserMenu(): JSX.Element {
  const me = useMe();
  const logout = useLogout();
  const navigate = useNavigate();
  const toast = useToast();
  const { preference, setPreference } = useTheme();
  const user = me.data;

  function signOut(): void {
    logout.mutate(undefined, {
      onSuccess: () => void navigate("/login", { replace: true }),
      onError: (error) => toast.error(describeError(error).message),
    });
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Account menu">
          <UserIcon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <div className="px-2.5 py-2">
          <p className="truncate text-sm font-medium">{user?.displayName || user?.email}</p>
          {user?.displayName ? <p className="truncate text-xs text-muted">{user.email}</p> : null}
          {user?.isRoot ? <p className="mt-0.5 text-xs text-muted">Root user</p> : null}
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild className="sm:hidden">
          <Link to="/docs">
            <BookOpen /> Docs
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild className="sm:hidden">
          <Link to="/settings">
            <Settings /> Settings
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSeparator className="sm:hidden" />
        <DropdownMenuLabel>Theme</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={preference} onValueChange={(value) => isPreference(value) && setPreference(value)}>
          <DropdownMenuRadioItem value="system">
            <Monitor /> System
          </DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="light">
            <Sun /> Light
          </DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="dark">
            <Moon /> Dark
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={signOut}>
          <LogOut /> Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
