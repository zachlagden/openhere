import { useRef, useState, type JSX, type KeyboardEvent, type PointerEvent } from "react";
import {
  areaPath,
  formatAxisNumber,
  indexAt,
  linePath,
  maxTicksFor,
  maxValue,
  niceCeiling,
  plotArea,
  roundedTopBar,
  summarise,
  tickIndices,
  xCentre,
  yFor,
  yTicks,
  type ChartPoint,
  type Layout,
} from "@/lib/chart";
import { formatCount } from "@/lib/format";
import { useElementWidth } from "@/lib/use-element-width";

const BAR_LIMIT = 60;
const MARKER_LIMIT = 31;

function Legend(): JSX.Element {
  return (
    <ul className="flex flex-wrap gap-x-5 gap-y-1 text-sm" aria-label="Chart legend">
      <li className="inline-flex items-center gap-2">
        <span aria-hidden="true" className="size-3 rounded-sm bg-chart-views" />
        Views
      </li>
      <li className="inline-flex items-center gap-2">
        <span aria-hidden="true" className="h-0.5 w-4 rounded-full bg-chart-uniques" />
        Unique visitors
      </li>
    </ul>
  );
}

interface PlotProps {
  points: ChartPoint[];
  layout: Layout;
  ceiling: number;
  active: number | null;
}

function Plot({ points, layout, ceiling, active }: PlotProps): JSX.Element {
  const { x0, x1, y1 } = plotArea(layout);
  const count = points.length;
  const slot = (x1 - x0) / Math.max(count, 1);
  const barWidth = Math.max(2, Math.min(36, slot * 0.62));
  const views = points.map((point) => point.views);
  const uniques = points.map((point) => point.uniques);
  const labelled = tickIndices(count, maxTicksFor(layout.width));
  const activePoint = active === null ? null : points[active];
  return (
    <g>
      {yTicks(ceiling).map((tick) => (
        <g key={tick}>
          <line x1={x0} x2={x1} y1={yFor(tick, ceiling, layout)} y2={yFor(tick, ceiling, layout)} strokeWidth={1} className="stroke-line" />
          <text x={x0 - 8} y={yFor(tick, ceiling, layout) + 4} textAnchor="end" fontSize={11}  className="fill-muted">
            {formatAxisNumber(tick)}
          </text>
        </g>
      ))}
      {count <= BAR_LIMIT ? (
        points.map((point) => (
          <path
            key={point.start}
            d={roundedTopBar(xCentre(point.index, count, layout) - barWidth / 2, yFor(point.views, ceiling, layout), barWidth, y1 - yFor(point.views, ceiling, layout), 4)}
            opacity={active === null || active === point.index ? 1 : 0.55} className="fill-chart-views" />
        ))
      ) : (
        <path d={areaPath(views, ceiling, layout)} opacity={0.35} strokeWidth={2} className="fill-chart-views stroke-chart-views" />
      )}
      <path d={linePath(uniques, ceiling, layout)} fill="none" strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" className="stroke-chart-uniques" />
      {count <= MARKER_LIMIT
        ? points.map((point) => (
            <circle key={point.start} cx={xCentre(point.index, count, layout)} cy={yFor(point.uniques, ceiling, layout)} r={4} strokeWidth={2} className="fill-chart-uniques stroke-surface" />
          ))
        : null}
      {labelled.map((index) => (
        <text key={index} x={xCentre(index, count, layout)} y={y1 + 18} textAnchor="middle" fontSize={11}  className="fill-muted">
          {points[index]?.label}
        </text>
      ))}
      {activePoint ? (
        <g>
          <line x1={xCentre(activePoint.index, count, layout)} x2={xCentre(activePoint.index, count, layout)} y1={layout.margins.top} y2={y1} strokeWidth={1} strokeDasharray="3 3" className="stroke-line-strong" />
          <circle cx={xCentre(activePoint.index, count, layout)} cy={yFor(activePoint.uniques, ceiling, layout)} r={5} strokeWidth={2} className="fill-chart-uniques stroke-surface" />
        </g>
      ) : null}
    </g>
  );
}

function Tooltip({ point, layout, count }: { point: ChartPoint; layout: Layout; count: number }): JSX.Element {
  const centre = xCentre(point.index, count, layout);
  const flip = centre > layout.width / 2;
  return (
    <div
      aria-hidden="true"
      data-testid="chart-tooltip"
      className="pointer-events-none absolute top-2 z-10 min-w-36 rounded-md border border-line bg-surface px-3 py-2 text-xs shadow-dialog"
      style={flip ? { right: layout.width - centre + 10 } : { left: centre + 10 }}
    >
      <p className="font-medium">{point.fullLabel}</p>
      <p className="mt-1 flex items-center justify-between gap-4">
        <span className="inline-flex items-center gap-1.5">
          <span className="size-2 rounded-sm bg-chart-views" />
          Views
        </span>
        <span className="tabular-nums">{formatCount(point.views)}</span>
      </p>
      <p className="flex items-center justify-between gap-4">
        <span className="inline-flex items-center gap-1.5">
          <span className="h-0.5 w-2 rounded-full bg-chart-uniques" />
          Unique visitors
        </span>
        <span className="tabular-nums">{formatCount(point.uniques)}</span>
      </p>
    </div>
  );
}

export function TrafficChart({ points }: { points: ChartPoint[] }): JSX.Element {
  const frame = useRef<HTMLDivElement>(null);
  const width = useElementWidth(frame);
  const [active, setActive] = useState<number | null>(null);
  const layout: Layout = { width, height: width < 480 ? 220 : 260, margins: { top: 12, right: 12, bottom: 28, left: 44 } };
  const ceiling = niceCeiling(maxValue(points));
  const count = points.length;
  const activePoint = active === null ? null : (points[active] ?? null);

  function onPointerMove(event: PointerEvent<HTMLDivElement>): void {
    const box = event.currentTarget.getBoundingClientRect();
    setActive(indexAt(event.clientX - box.left, count, layout));
  }

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
    const last = count - 1;
    const current = active ?? -1;
    const moves: Record<string, number | null> = {
      ArrowRight: Math.min(last, current + 1),
      ArrowLeft: Math.max(0, current === -1 ? last : current - 1),
      Home: 0,
      End: last,
      Escape: null,
    };
    if (event.key in moves) {
      event.preventDefault();
      setActive(moves[event.key] ?? null);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <Legend />
      <div
        ref={frame}
        role="group"
        tabIndex={0}
        aria-label="Views and unique visitors over time. Use the left and right arrow keys to read each point."
        className="relative rounded-md"
        onPointerMove={onPointerMove}
        onPointerLeave={() => setActive(null)}
        onKeyDown={onKeyDown}
        onBlur={() => setActive(null)}
      >
        <svg role="img" aria-label={summarise(points)} width={layout.width} height={layout.height} viewBox={`0 0 ${layout.width} ${layout.height}`} className="block max-w-full">
          <Plot points={points} layout={layout} ceiling={ceiling} active={active} />
        </svg>
        {activePoint ? <Tooltip point={activePoint} layout={layout} count={count} /> : null}
      </div>
      <p className="sr-only" aria-live="polite">
        {activePoint ? `${activePoint.fullLabel}: ${activePoint.views} views, ${activePoint.uniques} unique visitors.` : ""}
      </p>
    </div>
  );
}
