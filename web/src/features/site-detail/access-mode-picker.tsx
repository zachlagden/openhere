import { Globe, Lock, ShieldCheck } from "lucide-react";
import type { JSX } from "react";
import type { AccessMode } from "@/api/types";
import { cn } from "@/lib/cn";

const MODES: { mode: AccessMode; label: string; icon: JSX.Element; description: string }[] = [
  { mode: "public", label: "Public", icon: <Globe aria-hidden="true" />, description: "Anyone with the link can open the site." },
  {
    mode: "password",
    label: "Password",
    icon: <Lock aria-hidden="true" />,
    description: "Visitors must enter the site password. Changing the password signs every visitor out.",
  },
  {
    mode: "restricted",
    label: "Restricted",
    icon: <ShieldCheck aria-hidden="true" />,
    description:
      "Users of this server with at least read access can open it, and so can visitors whose email address or domain is on the lists below. With empty lists, only those users get in.",
  },
];

interface AccessModePickerProps {
  value: AccessMode;
  onChange: (mode: AccessMode) => void;
  disabled: boolean;
}

export function AccessModePicker({ value, onChange, disabled }: AccessModePickerProps): JSX.Element {
  return (
    <fieldset className="flex flex-col gap-2" disabled={disabled}>
      <legend className="mb-1 text-sm font-medium">Who can open this site</legend>
      {MODES.map((item) => (
        <label
          key={item.mode}
          className={cn(
            "flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2.5 has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-70",
            value === item.mode ? "border-accent bg-accent-soft" : "border-line hover:bg-sunken",
          )}
        >
          <input type="radio" name="access-mode" value={item.mode} checked={value === item.mode} onChange={() => onChange(item.mode)} className="mt-1 accent-[var(--accent)]" />
          <span className="min-w-0">
            <span className="flex items-center gap-1.5 text-sm font-medium [&_svg]:size-4">
              {item.icon}
              {item.label}
            </span>
            <span className="mt-0.5 block text-sm text-muted">{item.description}</span>
          </span>
        </label>
      ))}
    </fieldset>
  );
}
