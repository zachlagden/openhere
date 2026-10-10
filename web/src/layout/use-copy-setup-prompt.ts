import { describeError } from "@/api/errors";
import { useToast } from "@/components/ui/toast";
import { copyText } from "@/lib/clipboard";
import { buildSetupPrompt } from "@/lib/setup-prompt";
import { useInstance } from "@/queries/instance";
import { useMe } from "@/queries/session";

export function useCopySetupPrompt(): () => Promise<void> {
  const instance = useInstance();
  const me = useMe();
  const toast = useToast();

  return async () => {
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
    if (await copyText(prompt)) {
      toast.success("Agent setup prompt copied. Paste it into your agent.");
    } else {
      toast.error("Could not copy to the clipboard. Allow clipboard access and try again.");
    }
  };
}
