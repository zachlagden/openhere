import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";
import { readSelection, writeSelection, type Selection } from "@/lib/analytics-range";

export function useSelection(): { selection: Selection; setSelection: (next: Selection) => void } {
  const [search, setSearch] = useSearchParams();
  const selection = useMemo(() => readSelection(search, new Date()), [search]);
  const setSelection = useCallback(
    (next: Selection) => {
      setSearch((current) => writeSelection(current, next), { replace: true });
    },
    [setSearch],
  );
  return { selection, setSelection };
}
