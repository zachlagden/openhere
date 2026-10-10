import { useEffect } from "react";
import { BRAND_NAME } from "@/brand";

export function pageTitle(parts: (string | undefined | null)[]): string {
  const named = parts.filter((part): part is string => typeof part === "string" && part !== "");
  return [...named, BRAND_NAME].join(" | ");
}

export function usePageTitle(...parts: (string | undefined | null)[]): void {
  const title = pageTitle(parts);
  useEffect(() => {
    document.title = title;
  }, [title]);
}
