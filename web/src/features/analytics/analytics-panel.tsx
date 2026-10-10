import { ChartNoAxesColumn } from "lucide-react";
import { useState, type JSX } from "react";
import { describeError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { BlockSkeleton, LoadError } from "@/components/ui/query-status";
import { Section } from "@/components/ui/section";
import { toQuery } from "@/lib/analytics-range";
import { buildPoints } from "@/lib/chart";
import { formatCount } from "@/lib/format";
import { useAnalyticsReport, type AnalyticsScope } from "@/queries/analytics";
import { ChartTable } from "./chart-table";
import { RangeControls } from "./range-controls";
import { RankedLists } from "./ranked-lists";
import { Totals } from "./totals";
import { TrafficChart } from "./traffic-chart";
import { useSelection } from "./use-selection";

interface AnalyticsPanelProps {
  scope: AnalyticsScope;
  isRoot: boolean;
  recording?: { enabled: boolean; canChange: boolean; onEnable: () => void; pending: boolean; error: unknown };
}

function RecordingOff({ recording }: { recording: NonNullable<AnalyticsPanelProps["recording"]> }): JSX.Element {
  return (
    <EmptyState
      icon={<ChartNoAxesColumn />}
      title="Analytics is off for this site"
      actions={
        recording.canChange ? (
          <Button variant="primary" disabled={recording.pending} onClick={recording.onEnable}>
            {recording.pending ? "Turning on…" : "Turn on analytics"}
          </Button>
        ) : undefined
      }
    >
      <p>
        The server is not counting page views for this site. It counts anonymous views of HTML pages, with no cookies and no stored addresses. Anything recorded earlier stays below.
      </p>
      {recording.canChange ? null : <p className="mt-2">Someone with write access can turn it on.</p>}
      {recording.error ? <p className="mt-2 text-danger">{describeError(recording.error).message}</p> : null}
    </EmptyState>
  );
}

export function AnalyticsPanel({ scope, isRoot, recording }: AnalyticsPanelProps): JSX.Element {
  const { selection, setSelection } = useSelection();
  const report = useAnalyticsReport(scope, toQuery(selection));
  const [showTable, setShowTable] = useState(false);
  const off = recording !== undefined && !recording.enabled;

  return (
    <div className="flex flex-col gap-6">
      {off ? <RecordingOff recording={recording} /> : null}
      <RangeControls key={`${selection.preset}-${selection.from}-${selection.to}`} selection={selection} onChange={setSelection} />
      <div aria-live="polite" aria-busy={report.isFetching}>
        {report.isPending ? <BlockSkeleton label="Loading analytics" rows={4} /> : null}
        {report.isError && !report.data ? <LoadError error={report.error} onRetry={() => void report.refetch()} /> : null}
        {report.isError && report.data ? <Alert tone="error">{describeError(report.error).message}</Alert> : null}
        {report.data ? (
          <div className="flex flex-col gap-6" data-fetching={report.isFetching && !report.isPlaceholderData ? "true" : undefined}>
            {off && report.data.totals.views === 0 && report.data.totals.botViews === 0 ? null : <ReportBody report={report.data} isRoot={isRoot} showTable={showTable} onToggleTable={() => setShowTable((value) => !value)} />}
          </div>
        ) : null}
      </div>
    </div>
  );
}

interface ReportBodyProps {
  report: NonNullable<ReturnType<typeof useAnalyticsReport>["data"]>;
  isRoot: boolean;
  showTable: boolean;
  onToggleTable: () => void;
}

function ReportBody({ report, isRoot, showTable, onToggleTable }: ReportBodyProps): JSX.Element {
  const points = buildPoints(report);
  const empty = report.totals.views === 0 && report.totals.botViews === 0;
  return (
    <>
      <Totals totals={report.totals} />
      {typeof report.droppedEvents === "number" ? (
        <p className="text-sm text-muted">
          Events the server could not record since it started: <strong className="font-medium text-foreground">{formatCount(report.droppedEvents)}</strong>.
        </p>
      ) : null}
      <Section
        title={report.interval === "hour" ? "Views by hour" : "Views by day"}
        description="Times are in UTC."
        actions={
          <Button variant="secondary" size="sm" aria-pressed={showTable} onClick={onToggleTable}>
            {showTable ? "Hide data table" : "Show data table"}
          </Button>
        }
      >
        {empty ? <p className="text-sm text-muted">No page views were recorded in this range.</p> : <TrafficChart points={points} />}
        {showTable ? <ChartTable points={points} /> : null}
      </Section>
      <RankedLists report={report} isRoot={isRoot} />
    </>
  );
}
