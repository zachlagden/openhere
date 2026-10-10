export interface SitesParams {
  q: string;
  folder: string;
}

export const UNFILED_FOLDER = "none";

export function readSitesParams(search: URLSearchParams): SitesParams {
  return {
    q: (search.get("q") ?? "").trim(),
    folder: (search.get("folder") ?? "").trim(),
  };
}

export function writeSitesParams(current: URLSearchParams, patch: Partial<SitesParams>): URLSearchParams {
  const next = new URLSearchParams(current);
  for (const [key, value] of Object.entries(patch)) {
    const text = (value ?? "").trim();
    if (text === "") {
      next.delete(key);
    } else {
      next.set(key, text);
    }
  }
  return next;
}

export function hasActiveFilters(params: SitesParams): boolean {
  return params.q !== "" || params.folder !== "";
}

export function safeNextPath(raw: string | null | undefined): string {
  const fallback = "/";
  if (!raw || !raw.startsWith("/") || raw.startsWith("//") || raw.includes("\\")) {
    return fallback;
  }
  for (const char of raw) {
    const code = char.codePointAt(0) ?? 0;
    if (code < 0x20 || code === 0x7f) {
      return fallback;
    }
  }
  return raw;
}

export function isApiPath(path: string): boolean {
  return path === "/api" || path.startsWith("/api/");
}

export function loginPathFor(current: string): string {
  const next = safeNextPath(current);
  return next === "/" ? "/login" : `/login?next=${encodeURIComponent(next)}`;
}
