import { useId, type ComponentProps, type JSX, type ReactNode } from "react";
import { cn } from "@/lib/cn";

export function Input({ className, ...props }: ComponentProps<"input">): JSX.Element {
  return (
    <input
      className={cn(
        "h-9 w-full min-w-0 rounded-md border border-line-strong bg-surface px-3 text-sm text-foreground placeholder:text-muted/80 disabled:opacity-60",
        className,
      )}
      {...props}
    />
  );
}

export function Select({ className, ...props }: ComponentProps<"select">): JSX.Element {
  return (
    <select
      className={cn("h-9 w-full min-w-0 rounded-md border border-line-strong bg-surface px-2.5 text-sm text-foreground", className)}
      {...props}
    />
  );
}

export function Label({ className, ...props }: ComponentProps<"label">): JSX.Element {
  return <label className={cn("text-sm font-medium text-foreground", className)} {...props} />;
}

interface FieldFrameProps {
  id: string;
  label: string;
  hint?: string | undefined;
  error?: string | null | undefined;
  className?: string | undefined;
  children: ReactNode;
}

function FieldFrame({ id, label, hint, error, className, children }: FieldFrameProps): JSX.Element {
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <Label htmlFor={id}>{label}</Label>
      {children}
      {hint ? (
        <p id={`${id}-hint`} className="text-xs text-muted">
          {hint}
        </p>
      ) : null}
      {error ? (
        <p id={`${id}-error`} role="alert" className="text-xs text-danger">
          {error}
        </p>
      ) : null}
    </div>
  );
}

function describedBy(id: string, hint: string | undefined, error: string | null | undefined): string | undefined {
  const ids = [hint ? `${id}-hint` : "", error ? `${id}-error` : ""].filter(Boolean);
  return ids.length > 0 ? ids.join(" ") : undefined;
}

type TextFieldProps = Omit<ComponentProps<"input">, "id"> & {
  label: string;
  hint?: string;
  error?: string | null;
  wrapperClassName?: string;
};

export function TextField({ label, hint, error, wrapperClassName, ...props }: TextFieldProps): JSX.Element {
  const id = useId();
  return (
    <FieldFrame id={id} label={label} hint={hint} error={error} className={wrapperClassName}>
      <Input
        id={id}
        aria-describedby={describedBy(id, hint, error)}
        aria-invalid={error ? true : undefined}
        {...(props.type === "email" ? { spellCheck: false, autoCapitalize: "none" } : {})}
        {...props}
      />
    </FieldFrame>
  );
}

type SelectFieldProps = Omit<ComponentProps<"select">, "id"> & {
  label: string;
  hint?: string;
  error?: string | null;
};

export function SelectField({ label, hint, error, children, ...props }: SelectFieldProps): JSX.Element {
  const id = useId();
  return (
    <FieldFrame id={id} label={label} hint={hint} error={error}>
      <Select id={id} aria-describedby={describedBy(id, hint, error)} {...props}>
        {children}
      </Select>
    </FieldFrame>
  );
}

type TextAreaFieldProps = Omit<ComponentProps<"textarea">, "id"> & {
  label: string;
  hint?: string;
  error?: string | null;
};

export function TextAreaField({ label, hint, error, className, ...props }: TextAreaFieldProps): JSX.Element {
  const id = useId();
  return (
    <FieldFrame id={id} label={label} hint={hint} error={error}>
      <textarea
        id={id}
        aria-describedby={describedBy(id, hint, error)}
        aria-invalid={error ? true : undefined}
        className={cn("min-h-24 w-full min-w-0 rounded-md border border-line-strong bg-surface px-3 py-2 text-sm text-foreground", className)}
        {...props}
      />
    </FieldFrame>
  );
}
