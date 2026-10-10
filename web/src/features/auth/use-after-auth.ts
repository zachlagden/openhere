import { useCallback } from "react";
import { useNavigate } from "react-router";
import { isApiPath, safeNextPath } from "@/lib/query-state";

export function useAfterAuth(): (next: string | null | undefined) => void {
  const navigate = useNavigate();
  return useCallback(
    (next) => {
      const target = safeNextPath(next);
      if (isApiPath(target)) {
        window.location.assign(target);
        return;
      }
      void navigate(target, { replace: true });
    },
    [navigate],
  );
}
