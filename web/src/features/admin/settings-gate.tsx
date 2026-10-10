import type { JSX, ReactNode } from "react";
import type { Settings } from "@/api/types";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { useAdminSettings } from "@/queries/admin";

export function SettingsGate({ children }: { children: (settings: Settings) => ReactNode }): JSX.Element {
  const settings = useAdminSettings();
  if (settings.isPending) {
    return <BlockSkeleton label="Loading settings" rows={4} />;
  }
  if (settings.isError) {
    return <LoadError error={settings.error} onRetry={() => void settings.refetch()} />;
  }
  return <>{children(settings.data)}</>;
}
