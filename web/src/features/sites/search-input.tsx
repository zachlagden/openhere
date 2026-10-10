import { Search, X } from "lucide-react";
import { useEffect, useRef, type JSX } from "react";
import { Button } from "@/components/ui/button";
import { useFocusShortcut } from "@/lib/use-focus-shortcut";

export const SEARCH_DELAY_MS = 250;

interface SearchInputProps {
  value: string;
  onSearch: (value: string) => void;
}

export function SearchInput({ value, onSearch }: SearchInputProps): JSX.Element {
  const inputRef = useRef<HTMLInputElement>(null);
  useFocusShortcut(inputRef, "/");

  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  function cancel(): void {
    clearTimeout(timer.current);
    timer.current = undefined;
  }

  function schedule(text: string): void {
    cancel();
    timer.current = setTimeout(() => onSearch(text), SEARCH_DELAY_MS);
  }

  useEffect(() => () => clearTimeout(timer.current), []);

  useEffect(() => {
    const input = inputRef.current;
    if (input && document.activeElement !== input && input.value !== value) {
      input.value = value;
    }
  }, [value]);

  function clear(): void {
    cancel();
    if (inputRef.current) {
      inputRef.current.value = "";
      inputRef.current.focus();
    }
    onSearch("");
  }

  return (
    <div className="relative">
      <Search aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" />
      <input
        ref={inputRef}
        type="search"
        role="searchbox"
        aria-label="Search sites by name or address"
        placeholder="Search sites"
        defaultValue={value}
        autoComplete="off"
        spellCheck={false}
        onChange={(event) => schedule(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            cancel();
            onSearch(event.currentTarget.value);
          }
          if (event.key === "Escape" && event.currentTarget.value !== "") {
            event.preventDefault();
            clear();
          }
        }}
        className="h-10 w-full rounded-md border border-line-strong bg-surface pl-9 pr-16 text-sm [&::-webkit-search-cancel-button]:hidden"
      />
      {value !== "" ? (
        <Button variant="ghost" size="icon" className="absolute right-1 top-1/2 size-8 -translate-y-1/2" aria-label="Clear search" onClick={clear}>
          <X />
        </Button>
      ) : (
        <kbd
          aria-hidden="true"
          className="pointer-events-none absolute right-2.5 top-1/2 hidden -translate-y-1/2 rounded border border-line-strong bg-sunken px-1.5 font-mono text-xs text-muted sm:block"
        >
          /
        </kbd>
      )}
    </div>
  );
}
