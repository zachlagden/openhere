import { request } from "./client";
import { segment } from "./paths";
import type { ApiKey, CreatedApiKey } from "./types";

interface Signal {
  signal?: AbortSignal;
}

export async function listApiKeys({ signal }: Signal = {}): Promise<ApiKey[]> {
  const result = await request<{ keys: ApiKey[] }>("/me/keys", { signal });
  return result.keys;
}

export async function createApiKey(label: string): Promise<CreatedApiKey> {
  return request<CreatedApiKey>("/me/keys", { method: "POST", body: { label } });
}

export async function revokeApiKey(id: string): Promise<void> {
  await request<void>(`/me/keys/${segment(id)}`, { method: "DELETE" });
}
