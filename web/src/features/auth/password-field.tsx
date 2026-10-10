import { Eye, EyeOff } from "lucide-react";
import { useId, useState, type ComponentProps, type JSX } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/field";

type PasswordFieldProps = Omit<ComponentProps<"input">, "id" | "type"> & {
  label: string;
  hint?: string;
};

export function PasswordField({ label, hint, ...props }: PasswordFieldProps): JSX.Element {
  const id = useId();
  const [visible, setVisible] = useState(false);
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="relative">
        <Input id={id} type={visible ? "text" : "password"} aria-describedby={hint ? `${id}-hint` : undefined} className="pr-10" {...props} />
        <Button
          variant="ghost"
          size="icon"
          className="absolute inset-y-0 right-0 size-9 text-muted"
          aria-label={visible ? "Hide password" : "Show password"}
          aria-pressed={visible}
          onClick={() => setVisible((value) => !value)}
        >
          {visible ? <EyeOff /> : <Eye />}
        </Button>
      </div>
      {hint ? (
        <p id={`${id}-hint`} className="text-xs text-muted">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
