import type { JSX } from "react";
import { Navigate, Outlet, useLocation } from "react-router";
import { ApiDownPage } from "@/layout/api-down-page";
import { FullPageSpinner } from "@/layout/full-page-spinner";
import { loginPathFor } from "@/lib/query-state";
import { useMe, useSetupStatus } from "@/queries/session";

export function RequireAuth(): JSX.Element {
  const me = useMe();
  const location = useLocation();
  const signedOut = me.isSuccess && me.data === null;
  const setupStatus = useSetupStatus(signedOut);

  if (me.isPending) {
    return <FullPageSpinner />;
  }
  if (me.isError) {
    return <ApiDownPage error={me.error} onRetry={() => void me.refetch()} />;
  }
  if (me.data === null) {
    if (setupStatus.isPending) {
      return <FullPageSpinner />;
    }
    if (setupStatus.data) {
      return <Navigate to="/setup" replace />;
    }
    return <Navigate to={loginPathFor(location.pathname + location.search)} replace />;
  }
  return <Outlet />;
}
