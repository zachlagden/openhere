import { SKILL_INSTALL_COMMAND, SKILL_NAME } from "@/brand";

export interface SetupPromptInput {
  instanceUrl: string;
  emailAvailable: boolean;
  credentialsPath: string;
  userEmail: string;
}

function trimUrl(url: string): string {
  return url.replace(/\/+$/, "");
}

function signInLine(input: SetupPromptInput, base: string): string {
  if (input.emailAvailable) {
    return `3. Sign in with an email code. My account email is ${input.userEmail}. Request the code, ask me for the 8-digit code from my inbox, then verify it. The key is saved to ${input.credentialsPath}.`;
  }
  return `3. This instance cannot send email. Ask me to create an API key at ${base}/settings/api-keys (Settings, API keys) and give it to you. Save it to ${input.credentialsPath}.`;
}

export function buildSetupPrompt(input: SetupPromptInput): string {
  const base = trimUrl(input.instanceUrl);
  const lines = [
    `Publish static sites for me to my self-hosted OpenHere instance.`,
    "",
    `1. Install the ${SKILL_NAME} skill: ${SKILL_INSTALL_COMMAND}`,
    `2. My instance URL is ${base}. Pass it to the skill's scripts as --instance.`,
    signInLine(input, base),
    `4. If you cannot install skills, sign in the same way, then read the agent guide at ${base}/api/v1/docs/agent-guide.md (it needs the key) and publish with the API as it describes.`,
    "",
    "Never print, log or commit the key. Publish only folders that are already built, and ask me before you overwrite an existing site.",
  ];
  return lines.join("\n");
}
