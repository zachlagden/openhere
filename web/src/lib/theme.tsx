import { createContext, use, useCallback, useEffect, useMemo, useState, type JSX, type ReactNode } from "react";

export type ThemePreference = "system" | "light" | "dark";

const STORAGE_KEY = "theme";

interface ThemeApi {
  preference: ThemePreference;
  setPreference: (preference: ThemePreference) => void;
}

const ThemeContext = createContext<ThemeApi | null>(null);

function readPreference(): ThemePreference {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return stored === "light" || stored === "dark" ? stored : "system";
  } catch {
    return "system";
  }
}

export function applyTheme(preference: ThemePreference): void {
  const dark = preference === "dark" || (preference === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
  document.documentElement.style.colorScheme = dark ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }): JSX.Element {
  const [preference, setPreferenceState] = useState<ThemePreference>(readPreference);

  const setPreference = useCallback((next: ThemePreference): void => {
    setPreferenceState(next);
    applyTheme(next);
    try {
      window.localStorage.setItem(STORAGE_KEY, next);
    } catch {
      return;
    }
  }, []);

  useEffect(() => {
    if (preference !== "system") {
      return undefined;
    }
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const follow = (): void => applyTheme("system");
    query.addEventListener("change", follow);
    return () => query.removeEventListener("change", follow);
  }, [preference]);

  const api = useMemo<ThemeApi>(() => ({ preference, setPreference }), [preference, setPreference]);
  return <ThemeContext value={api}>{children}</ThemeContext>;
}

export function useTheme(): ThemeApi {
  const api = use(ThemeContext);
  if (!api) {
    throw new Error("useTheme needs a ThemeProvider.");
  }
  return api;
}
