import type { JSX } from "react";
import { NavLink } from "react-router";
import { cn } from "@/lib/cn";

export interface TabItem {
  to: string;
  label: string;
  end?: boolean;
}

export function TabNav({ label, items }: { label: string; items: TabItem[] }): JSX.Element {
  return (
    <nav aria-label={label} className="-mx-4 mb-6 overflow-x-auto border-b border-line px-4 sm:mx-0 sm:px-0">
      <ul className="flex min-w-max gap-1">
        {items.map((item) => (
          <li key={item.to}>
            <NavLink
              to={item.to}
              end={item.end ?? false}
              className={({ isActive }) =>
                cn(
                  "-mb-px inline-block whitespace-nowrap border-b-2 px-3 py-2.5 text-sm",
                  isActive ? "border-accent font-medium text-foreground" : "border-transparent text-muted hover:text-foreground",
                )
              }
            >
              {item.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
