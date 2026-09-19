"use client";

import { useId, useRef, useState } from "react";
import { cn } from "@/lib/utils";

export interface TrendPoint {
  /** UTC calendar day, YYYY-MM-DD — the x position and the tooltip's heading. */
  day: string;
  value: number;
}

const VB_W = 640;
const VB_H = 180;
const PAD_L = 8;
const PAD_R = 8;
const PAD_T = 12;
const PAD_B = 22;

/**
 * One measure over time: a 2px line over a 10% wash, the endpoint labelled, a
 * crosshair that snaps to the nearest day, and the same readout on keyboard
 * focus (← →). One series, so there is no legend — the title names it.
 *
 * The SVG scales to its container, so strokes carry `vector-effect` to stay
 * exactly 2px and the pointer maps through the rendered box, not the viewBox.
 */
export function TrendChart({
  points,
  title,
  format,
  formatDay,
  className,
}: {
  points: TrendPoint[];
  title: string;
  /** renders a value for the tooltip, the endpoint label and the axis top */
  format: (value: number) => string;
  formatDay: (day: string) => string;
  className?: string;
}) {
  const clipId = useId();
  const svgRef = useRef<SVGSVGElement>(null);
  const [cursor, setCursor] = useState<number | null>(null);

  if (points.length === 0) return null;

  const max = Math.max(...points.map((p) => p.value));
  // A flat-zero series still needs a scale; 1 keeps the baseline at the bottom.
  const top = max <= 0 ? 1 : max;
  const innerW = VB_W - PAD_L - PAD_R;
  const innerH = VB_H - PAD_T - PAD_B;
  const x = (i: number) =>
    points.length === 1 ? PAD_L + innerW / 2 : PAD_L + (i * innerW) / (points.length - 1);
  const y = (v: number) => PAD_T + innerH - (v / top) * innerH;

  const line = points.map((p, i) => `${i === 0 ? "M" : "L"}${x(i)},${y(p.value)}`).join(" ");
  const area = `${line} L${x(points.length - 1)},${PAD_T + innerH} L${x(0)},${PAD_T + innerH} Z`;

  const last = points[points.length - 1];
  const active = cursor === null ? null : points[cursor];

  function moveTo(clientX: number) {
    const box = svgRef.current?.getBoundingClientRect();
    if (!box || box.width === 0) return;
    const ratio = (clientX - box.left) / box.width;
    const inner = (ratio * VB_W - PAD_L) / innerW;
    const i = Math.round(inner * (points.length - 1));
    setCursor(Math.min(points.length - 1, Math.max(0, i)));
  }

  return (
    <figure className={cn("relative m-0 border border-border bg-card p-4", className)}>
      <figcaption className="flex items-baseline justify-between gap-2">
        <span className="text-sm font-bold">{title}</span>
        <span className="text-xs text-muted-foreground">
          {format(points.reduce((sum, p) => sum + p.value, 0))}
        </span>
      </figcaption>

      <svg
        ref={svgRef}
        viewBox={`0 0 ${VB_W} ${VB_H}`}
        className="mt-3 h-[180px] w-full touch-none outline-none focus-visible:ring-3 focus-visible:ring-ring/40"
        role="img"
        aria-label={title}
        tabIndex={0}
        onPointerMove={(e) => moveTo(e.clientX)}
        onPointerLeave={() => setCursor(null)}
        onFocus={() => setCursor(points.length - 1)}
        onBlur={() => setCursor(null)}
        onKeyDown={(e) => {
          if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
          e.preventDefault();
          setCursor((c) => {
            const from = c ?? points.length - 1;
            const next = from + (e.key === "ArrowRight" ? 1 : -1);
            return Math.min(points.length - 1, Math.max(0, next));
          });
        }}
      >
        <defs>
          <clipPath id={clipId}>
            <rect x={PAD_L} y={PAD_T} width={innerW} height={innerH} />
          </clipPath>
        </defs>

        {/* recessive grid: baseline, midpoint, top of the scale */}
        {[0, 0.5, 1].map((f) => (
          <line
            key={f}
            x1={PAD_L}
            x2={VB_W - PAD_R}
            y1={PAD_T + innerH * f}
            y2={PAD_T + innerH * f}
            stroke="var(--chart-grid)"
            strokeWidth={1}
            vectorEffect="non-scaling-stroke"
          />
        ))}

        <g clipPath={`url(#${clipId})`}>
          <path d={area} fill="var(--chart-series)" fillOpacity={0.1} />
          <path
            d={line}
            fill="none"
            stroke="var(--chart-series)"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
            vectorEffect="non-scaling-stroke"
          />
        </g>

        {active && (
          <line
            x1={x(cursor!)}
            x2={x(cursor!)}
            y1={PAD_T}
            y2={PAD_T + innerH}
            stroke="var(--chart-grid)"
            strokeWidth={1}
            vectorEffect="non-scaling-stroke"
          />
        )}

        {/* the endpoint carries the only direct label; the rest live in the tooltip */}
        <circle
          cx={x(points.length - 1)}
          cy={y(last.value)}
          r={4}
          fill="var(--chart-series)"
          stroke="var(--card)"
          strokeWidth={2}
          vectorEffect="non-scaling-stroke"
        />
        {active && cursor !== points.length - 1 && (
          <circle
            cx={x(cursor!)}
            cy={y(active.value)}
            r={4}
            fill="var(--chart-series)"
            stroke="var(--card)"
            strokeWidth={2}
            vectorEffect="non-scaling-stroke"
          />
        )}

        <text
          x={PAD_L}
          y={PAD_T - 3}
          className="fill-muted-foreground"
          style={{ fontSize: 11 }}
        >
          {format(top)}
        </text>

        <text
          x={PAD_L}
          y={VB_H - 6}
          className="fill-muted-foreground"
          style={{ fontSize: 11 }}
        >
          {formatDay(points[0].day)}
        </text>
        <text
          x={VB_W - PAD_R}
          y={VB_H - 6}
          textAnchor="end"
          className="fill-muted-foreground"
          style={{ fontSize: 11 }}
        >
          {formatDay(last.day)}
        </text>
      </svg>

      {/* The readout follows the crosshair: value leads, day follows. With no
          pointer it sits under the chart so the last value is always visible. */}
      {active ? (
        <div
          role="status"
          className="pointer-events-none absolute z-10 -translate-x-1/2 border border-border bg-popover px-2 py-1 text-xs whitespace-nowrap shadow-card"
          style={{
            // the svg spans the figure minus its 1rem padding on each side, and
            // the ratio is nudged off the edges so the bubble never overhangs
            left: `calc(1rem + ${Math.min(0.9, Math.max(0.1, x(cursor!) / VB_W)).toFixed(4)} * (100% - 2rem))`,
            top: "3.25rem",
          }}
        >
          <span className="font-bold text-foreground">{format(active.value)}</span>{" "}
          <span className="text-muted-foreground">{formatDay(active.day)}</span>
        </div>
      ) : (
        <div className="mt-1 flex items-baseline gap-2 text-xs">
          <span className="font-bold text-foreground">{format(last.value)}</span>
          <span className="text-muted-foreground">{formatDay(last.day)}</span>
        </div>
      )}
    </figure>
  );
}
