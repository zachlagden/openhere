import { BookOpen, Settings } from "lucide-react";
import type { JSX } from "react";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { WorkspaceSwitcher } from "@/features/workspaces/workspace-switcher";
import { CopyPromptButton } from "./copy-prompt-button";
import { Logo } from "./logo";
import { ThemeToggle } from "./theme-toggle";
import { UserMenu } from "./user-menu";

export function TopBar(): JSX.Element {
  return (
    <header className="sticky top-0 z-30 border-b border-line bg-background/90 backdrop-blur">
      <div className="mx-auto flex h-14 w-full max-w-7xl items-center gap-2 px-4 sm:gap-3 sm:px-6">
        <div className="hidden sm:block">
          <Logo />
        </div>
        <Link to="/" aria-label="Home" className="sm:hidden">
          <span aria-hidden="true" className="flex size-7 flex-col justify-center gap-[3px] rounded-[7px] bg-accent px-[6px]">
            <span className="h-[2.5px] rounded-full bg-accent-foreground" />
            <span className="h-[2.5px] rounded-full bg-accent-foreground" />
            <span className="h-[2.5px] w-2/3 rounded-full bg-accent-foreground" />
          </span>
        </Link>
        <span aria-hidden="true" className="hidden h-5 w-px bg-line-strong sm:block" />
        <WorkspaceSwitcher />
        <div className="ml-auto flex items-center gap-1">
          <CopyPromptButton />
          <Button asChild variant="ghost" size="icon" className="hidden sm:inline-flex">
            <Link to="/docs" aria-label="Docs" title="Docs">
              <BookOpen />
            </Link>
          </Button>
          <Button asChild variant="ghost" size="icon" className="hidden sm:inline-flex">
            <Link to="/settings" aria-label="Settings" title="Settings">
              <Settings />
            </Link>
          </Button>
          <span className="hidden sm:inline-flex">
            <ThemeToggle />
          </span>
          <UserMenu />
        </div>
      </div>
    </header>
  );
}
