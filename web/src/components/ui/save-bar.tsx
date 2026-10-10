import type { JSX } from "react";
import { Button } from "./button";

interface SaveBarProps {
  dirty: boolean;
  pending: boolean;
  onDiscard: () => void;
  saveLabel?: string;
}

export function SaveBar({ dirty, pending, onDiscard, saveLabel = "Save changes" }: SaveBarProps): JSX.Element {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button type="submit" variant="primary" disabled={!dirty || pending}>
        {pending ? "Saving…" : saveLabel}
      </Button>
      <Button variant="ghost" disabled={!dirty || pending} onClick={onDiscard}>
        Discard changes
      </Button>
    </div>
  );
}
