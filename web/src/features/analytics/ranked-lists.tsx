import type { JSX } from "react";
import { Link } from "react-router";
import type { Analytics } from "@/api/types";
import { RankedList } from "./ranked-list";

function LocationNote({ available, isRoot }: { available: boolean; isRoot: boolean }): JSX.Element | null {
  if (available) {
    return null;
  }
  if (isRoot) {
    return (
      <p>
        Location needs a MaxMind account.{" "}
        <Link to="/admin/analytics" className="font-medium text-foreground underline underline-offset-4">
          Add one in analytics settings
        </Link>
        .
      </p>
    );
  }
  return <p>Location is not set up on this server. Ask the administrator to add a MaxMind account.</p>;
}

export function RankedLists({ report, isRoot }: { report: Analytics; isRoot: boolean }): JSX.Element {
  const locationNote = <LocationNote available={report.locationAvailable} isRoot={isRoot} />;
  return (
    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      <RankedList title="Pages" items={report.paths} />
      <RankedList title="Referrers" items={report.referrers} emptyText="No referrers. Direct visits are not listed." />
      <RankedList title="Countries" items={report.countries} note={locationNote} />
      <RankedList title="Cities" items={report.cities} note={locationNote} />
      <RankedList title="Browsers" items={report.browsers} />
      <RankedList title="Operating systems" items={report.operatingSystems} />
      <RankedList title="Devices" items={report.devices} />
      <RankedList title="Status classes" items={report.statuses} />
    </div>
  );
}
