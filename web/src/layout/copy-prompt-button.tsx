import type { JSX } from "react";
import { CopyLabelButton } from "@/components/ui/copy-button";
import { useCopySetupPrompt } from "./use-copy-setup-prompt";

export function CopyPromptButton(): JSX.Element {
  const { status, copy } = useCopySetupPrompt();
  return (
    <CopyLabelButton
      variant="secondary"
      size="sm"
      label="Copy agent setup prompt"
      title="Copy a prompt that sets up your agent"
      status={status}
      onCopy={copy}
      labelClassName="hidden md:grid"
    />
  );
}
