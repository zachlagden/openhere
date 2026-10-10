import { ClipboardCopy } from "lucide-react";
import type { JSX } from "react";
import { Button } from "@/components/ui/button";
import { useCopySetupPrompt } from "./use-copy-setup-prompt";

export function CopyPromptButton(): JSX.Element {
  const copy = useCopySetupPrompt();
  return (
    <Button variant="secondary" size="sm" onClick={() => void copy()} aria-label="Copy agent setup prompt" title="Copy a prompt that sets up your agent">
      <ClipboardCopy />
      <span className="hidden md:inline">Copy agent setup prompt</span>
    </Button>
  );
}
