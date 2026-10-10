import type { JSX } from "react";
import { BehaviourSection } from "./behaviour-section";
import { DeleteSiteSection } from "./delete-site-section";
import { DetailsSection } from "./details-section";
import { FolderSection } from "./folder-section";
import { useSiteContext } from "./site-context";

export function OverviewTab(): JSX.Element {
  const { site } = useSiteContext();
  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <DetailsSection site={site} />
      <BehaviourSection site={site} />
      <FolderSection site={site} />
      <DeleteSiteSection site={site} />
    </div>
  );
}
