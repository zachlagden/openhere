import { useOutletContext } from "react-router";
import type { Site } from "@/api/types";

export interface SiteContext {
  site: Site;
  isRoot: boolean;
}

export function useSiteContext(): SiteContext {
  return useOutletContext<SiteContext>();
}
