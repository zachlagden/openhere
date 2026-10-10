export function segment(value: string): string {
  return encodeURIComponent(value);
}

export function sitePath(slug: string): string {
  return `/sites/${segment(slug)}`;
}

export function workspacePath(workspace: string): string {
  return `/workspaces/${segment(workspace)}`;
}
