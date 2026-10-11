import { describeError } from "@/api/errors";
import { useToast } from "@/components/ui/toast";
import { copyText } from "@/lib/clipboard";
import { buildSetupPrompt } from "@/lib/setup-prompt";
import { useCopyStatus, type CopyStatus } from "@/lib/use-copy-status";
import { useInstance } from "@/queries/instance";
import { useMe } from "@/queries/session";

interface CopySetupPrompt {
  status: CopyStatus;
  copy: () => void;
}

export function useCopySetupPrompt(): CopySetupPrompt {
  const instance = useInstance();
  const me = useMe();
  const toast = useToast();
  const { status, run } = useCopyStatus();

  function copy(): void {
    if (!instance.data || !me.data) {
      toast.error(instance.error ? describeError(instance.error).message : "The server details are still loading. Try again in a moment.");
      return;
    }
    const prompt = buildSetupPrompt({
      instanceUrl: instance.data.publicUrl,
      emailAvailable: instance.data.emailAvailable,
      credentialsPath: instance.data.agentCredentialsPath,
      userEmail: me.data.email,
    });
    void run(() => copyText(prompt));
  }

  return { status, copy };
}
