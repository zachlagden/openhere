export interface SetupPromptInput {
  instanceUrl: string;
  emailAvailable: boolean;
  credentialsPath: string;
  userEmail: string;
}

function trimUrl(url: string): string {
  return url.replace(/\/+$/, "");
}

function authenticationSteps(input: SetupPromptInput, base: string): string[] {
  if (input.emailAvailable) {
    return [
      `   - Ask me to approve you: POST ${base}/api/v1/auth/agent/request-code with {"email": "${input.userEmail}"}.`,
      "   - I will give you the 8-digit code from my email.",
      `   - POST ${base}/api/v1/auth/agent/verify-code with {"email": "${input.userEmail}", "code": "<code>", "label": "<your agent name>"}. The response holds an API key once.`,
      `   - Save {"url": "${base}", "key": "<key>"} to ${input.credentialsPath} (directory mode 0700, file mode 0600).`,
    ];
  }
  return [
    `   - This instance cannot send email. Ask me for an API key. I create one in the dashboard at ${base}/settings/api-keys.`,
    `   - Save {"url": "${base}", "key": "<key>"} to ${input.credentialsPath} (directory mode 0700, file mode 0600).`,
  ];
}

export function buildSetupPrompt(input: SetupPromptInput): string {
  const base = trimUrl(input.instanceUrl);
  const lines = [
    `You can publish static sites for me to my self-hosted hosting service at ${base}.`,
    "",
    "1. Authenticate.",
    `   - First read ${input.credentialsPath}. If it exists, use its "url" and "key" and send the key as "Authorization: Bearer <key>".`,
    "   - If it does not exist, get a key:",
    ...authenticationSteps(input, base),
    "   - Never print, log or commit the key.",
    "",
    "2. Read the agent guide, which needs the key:",
    `   GET ${base}/api/v1/docs/agent-guide.md`,
    "",
    "3. To publish a built folder:",
    `   - POST ${base}/api/v1/publish with every file's path, size, SHA-256 and content type (use PUT /api/v1/publish/{slug} for an existing site).`,
    "   - PUT each file's bytes to the upload URL the response lists.",
    "   - POST the finalize URL, then tell me the site address from the response.",
    "",
    "Publish only folders that are already built. Ask me before you overwrite an existing site.",
  ];
  return lines.join("\n");
}
