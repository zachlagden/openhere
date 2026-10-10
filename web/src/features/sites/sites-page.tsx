import type { JSX } from "react";
import { describeError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { PageHeader } from "@/components/ui/page-header";
import { usePageTitle } from "@/lib/use-page-title";
import { useMe } from "@/queries/session";
import { useWorkspaces } from "@/queries/workspaces";
import { SitesView } from "./sites-view";

export function SitesPage(): JSX.Element {
  const workspaces = useWorkspaces();
  const me = useMe();
  usePageTitle("Sites");

  if (workspaces.isError) {
    return <Alert tone="error">{describeError(workspaces.error).message}</Alert>;
  }

  return (
    <>
      <PageHeader title="Sites" description="Every site you can reach, across all your workspaces." />
      <SitesView workspace={undefined} workspaces={workspaces.data ?? []} loadingWorkspaces={workspaces.isPending} isRoot={me.data?.isRoot ?? false} />
    </>
  );
}
