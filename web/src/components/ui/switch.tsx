import { useId, type ComponentProps, type JSX, type ReactNode } from "react";
import { cn } from "@/lib/cn";

type SwitchFieldProps = Omit<ComponentProps<"input">, "id" | "type" | "role"> & {
  label: string;
  description?: ReactNode;
};

export function SwitchField({ label, description, className, ...props }: SwitchFieldProps): JSX.Element {
  const id = useId();
  return (
    <div className={cn("flex items-start justify-between gap-4", className)}>
      <div className="min-w-0">
        <label htmlFor={id} className="text-sm font-medium">
          {label}
        </label>
        {description ? (
          <p id={`${id}-description`} className="mt-0.5 text-sm text-muted">
            {description}
          </p>
        ) : null}
      </div>
      <input
        id={id}
        type="checkbox"
        role="switch"
        aria-describedby={description ? `${id}-description` : undefined}
        className="relative mt-0.5 h-6 w-11 shrink-0 cursor-pointer appearance-none rounded-full border border-line-strong bg-sunken transition-colors before:absolute before:left-0.5 before:top-0.5 before:size-4 before:rounded-full before:bg-muted before:transition-transform checked:border-accent checked:bg-accent checked:before:translate-x-5 checked:before:bg-accent-foreground disabled:cursor-not-allowed disabled:opacity-50"
        {...props}
      />
    </div>
  );
}
