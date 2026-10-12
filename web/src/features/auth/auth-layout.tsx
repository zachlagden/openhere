import type { JSX, ReactNode } from "react";
import { BRAND_NAME, REPOSITORY_URL } from "@/brand";
import { Logo } from "@/layout/logo";
import { ThemeToggle } from "@/layout/theme-toggle";
import { usePageTitle } from "@/lib/use-page-title";

interface AuthLayoutProps {
  title: string;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
}

export function AuthLayout({ title, description, children, footer }: AuthLayoutProps): JSX.Element {
  usePageTitle(title);
  return (
    <div className="grid min-h-dvh lg:grid-cols-[1.05fr_1fr]">
      <aside className="dot-grid relative hidden overflow-hidden border-r border-line bg-sunken lg:flex lg:flex-col lg:justify-between lg:p-12">
        <Logo />
        <div className="max-w-md">
          <p className="font-display text-5xl font-medium leading-[1.05] tracking-tight">
            Sites your agents publish, served from your own server.
          </p>
          <div className="mt-10 rounded-lg border border-line-strong bg-surface p-4 font-mono text-sm shadow-dialog" aria-hidden="true">
            <div className="text-muted">$ publish ./dist</div>
            <div className="mt-1 text-ok">live at sites.example.com/s/quiet-meadow-k3p7/</div>
          </div>
        </div>
        <p className="text-xs text-muted">
          <a href={REPOSITORY_URL} target="_blank" rel="noopener noreferrer" className="underline-offset-4 hover:text-foreground hover:underline">
            {BRAND_NAME} is open source and runs on your own server.
          </a>
        </p>
      </aside>
      <main className="flex min-w-0 flex-col px-4 py-6 sm:px-10">
        <div className="flex items-center justify-between lg:justify-end">
          <div className="lg:hidden">
            <Logo />
          </div>
          <ThemeToggle />
        </div>
        <div className="mx-auto flex w-full max-w-sm flex-1 flex-col justify-center py-10">
          <h1 className="font-display text-3xl font-medium tracking-tight">{title}</h1>
          {description ? <div className="mt-2 text-sm text-muted">{description}</div> : null}
          <div className="mt-7">{children}</div>
          {footer ? <div className="mt-6 text-sm text-muted">{footer}</div> : null}
        </div>
      </main>
    </div>
  );
}
