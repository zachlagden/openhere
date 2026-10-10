import type { JSX } from "react";
import { Outlet } from "react-router";
import { useRouteFocus } from "@/lib/use-route-focus";
import { ConnectionBanner } from "./connection-banner";
import { TopBar } from "./top-bar";

export function AppShell(): JSX.Element {
  useRouteFocus("main");
  return (
    <div className="flex min-h-dvh flex-col">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 focus:rounded-md focus:bg-surface focus:px-3 focus:py-2 focus:text-sm focus:shadow-dialog"
      >
        Skip to content
      </a>
      <ConnectionBanner />
      <TopBar />
      <main id="main" tabIndex={-1} className="outline-none mx-auto w-full max-w-7xl flex-1 px-4 py-8 sm:px-6">
        <Outlet />
      </main>
    </div>
  );
}
