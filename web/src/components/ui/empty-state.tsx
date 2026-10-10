import type { JSX, ReactNode } from "react";

interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  children?: ReactNode;
  actions?: ReactNode;
}

export function EmptyState({ icon, title, children, actions }: EmptyStateProps): JSX.Element {
  return (
    <div className="dot-grid rise flex flex-col items-center rounded-lg border border-dashed border-line-strong px-6 py-14 text-center">
      <div className="mb-4 flex size-11 items-center justify-center rounded-full border border-line bg-surface text-muted [&_svg]:size-5">
        {icon}
      </div>
      <h2 className="font-display text-xl font-medium">{title}</h2>
      {children ? <div className="mt-2 max-w-md text-sm text-muted">{children}</div> : null}
      {actions ? <div className="mt-5 flex flex-wrap justify-center gap-2">{actions}</div> : null}
    </div>
  );
}
