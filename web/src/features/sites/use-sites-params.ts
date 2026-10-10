import { useCallback } from "react";
import { useSearchParams } from "react-router";
import { readSitesParams, writeSitesParams, type SitesParams } from "@/lib/query-state";

export function useSitesParams(): { params: SitesParams; update: (patch: Partial<SitesParams>) => void } {
  const [search, setSearch] = useSearchParams();
  const params = readSitesParams(search);
  const update = useCallback(
    (patch: Partial<SitesParams>) => {
      setSearch((current) => writeSitesParams(current, patch), { replace: true });
    },
    [setSearch],
  );
  return { params, update };
}
