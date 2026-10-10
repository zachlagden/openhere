import { Monitor, Moon, Sun } from "lucide-react";
import type { JSX } from "react";
import { Button } from "@/components/ui/button";
import { useTheme, type ThemePreference } from "@/lib/theme";

const NEXT: Record<ThemePreference, ThemePreference> = { system: "light", light: "dark", dark: "system" };
const LABELS: Record<ThemePreference, string> = { system: "system", light: "light", dark: "dark" };

export function ThemeToggle(): JSX.Element {
  const { preference, setPreference } = useTheme();
  const Icon = preference === "system" ? Monitor : preference === "light" ? Sun : Moon;
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={`Theme: ${LABELS[preference]}. Switch to ${LABELS[NEXT[preference]]}.`}
      title={`Theme: ${LABELS[preference]}`}
      onClick={() => setPreference(NEXT[preference])}
    >
      <Icon />
    </Button>
  );
}
