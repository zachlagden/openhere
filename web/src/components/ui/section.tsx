import type { JSX, ReactNode } from "react";
import { cn } from "@/lib/cn";

interface SectionProps {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  headingLevel?: 2 | 3;
}

export function Section({ title, description, actions, children, className, headingLevel = 2 }: SectionProps): JSX.Element {
  const Heading = headingLevel === 2 ? "h2" : "h3";
  return (
    <section className={cn("rounded-lg border border-line bg-surface", className)}>
      <header className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2 border-b border-line px-4 py-3.5 sm:px-5">
        <div className="min-w-0">
          <Heading className="font-display text-lg font-medium leading-snug">{title}</Heading>
          {description ? <div className="mt-0.5 max-w-2xl text-sm text-muted">{description}</div> : null}
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </header>
      <div className="flex flex-col gap-4 px-4 py-4 sm:px-5">{children}</div>
    </section>
  );
}
