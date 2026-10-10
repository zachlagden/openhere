import { useEffect, useRef } from "react";
import { useLocation } from "react-router";

export function useRouteFocus(targetId: string): void {
  const { pathname } = useLocation();
  const previous = useRef(pathname);
  useEffect(() => {
    if (previous.current === pathname) {
      return;
    }
    previous.current = pathname;
    document.getElementById(targetId)?.focus({ preventScroll: true });
  }, [pathname, targetId]);
}
