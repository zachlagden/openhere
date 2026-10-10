import { X } from "lucide-react";
import { useState, type JSX, type KeyboardEvent } from "react";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/field";
import { addEntry, MAX_ALLOWLIST_ENTRIES, removeEntry, type AllowlistKind } from "@/lib/allowlist";

interface AllowlistEditorProps {
  kind: AllowlistKind;
  entries: string[];
  onChange: (entries: string[]) => void;
  disabled: boolean;
}

const COPY: Record<AllowlistKind, { title: string; label: string; placeholder: string; hint: string; empty: string }> = {
  email: {
    title: "Allowed email addresses",
    label: "Add an email address",
    placeholder: "sam@example.com",
    hint: "These visitors get an emailed code on the sign-in page.",
    empty: "No addresses yet.",
  },
  domain: {
    title: "Allowed email domains",
    label: "Add a domain",
    placeholder: "example.com",
    hint: "Matches addresses at exactly this domain, not its subdomains.",
    empty: "No domains yet.",
  },
};

export function AllowlistEditor({ kind, entries, onChange, disabled }: AllowlistEditorProps): JSX.Element {
  const [text, setText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const copy = COPY[kind];

  function add(): void {
    const result = addEntry(entries, kind, text);
    if (!result.ok) {
      setError(result.message);
      return;
    }
    setError(null);
    setText("");
    onChange(result.list);
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>): void {
    if (event.key === "Enter") {
      event.preventDefault();
      add();
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between gap-2">
        <h3 className="text-sm font-medium">{copy.title}</h3>
        <span className="text-xs text-muted">
          {entries.length} of {MAX_ALLOWLIST_ENTRIES}
        </span>
      </div>
      {entries.length === 0 ? (
        <p className="text-sm text-muted">{copy.empty}</p>
      ) : (
        <ul aria-label={copy.title} className="flex flex-wrap gap-1.5">
          {entries.map((entry) => (
            <li key={entry} className="inline-flex items-center gap-1 rounded-full border border-line bg-sunken py-0.5 pl-3 pr-1 font-mono text-xs">
              {entry}
              {disabled ? null : (
                <Button variant="ghost" size="icon" className="size-6 rounded-full" aria-label={`Remove ${entry}`} onClick={() => onChange(removeEntry(entries, entry))}>
                  <X />
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
      {disabled ? null : (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start">
          <div className="sm:w-80">
            <TextField label={copy.label} value={text} onChange={(event) => setText(event.target.value)} onKeyDown={onKeyDown} placeholder={copy.placeholder} hint={copy.hint} error={error} autoComplete="off" spellCheck={false} />
          </div>
          <Button variant="secondary" className="sm:mt-6" disabled={text.trim() === ""} onClick={add}>
            Add
          </Button>
        </div>
      )}
    </div>
  );
}
