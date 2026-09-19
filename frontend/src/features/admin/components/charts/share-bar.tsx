"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";

export interface Share {
  key: string;
  label: string;
  value: number;
  /** a CSS colour — a chart token, never a brand colour */
  color: string;
}

/**
 * Part-to-whole as one horizontal stacked bar: plain HTML, 2px surface gaps
 * doing the separating (no strokes), and a legend that carries every label and
 * value so identity is never colour-alone. Hovering or focusing a segment
 * lifts it and names it above the bar.
 */
export function ShareBar({
  shares,
  title,
  empty,
  formatValue,
  className,
}: {
  shares: Share[];
  title: string;
  /** shown when everything is zero */
  empty: string;
  formatValue: (value: number) => string;
  className?: string;
}) {
  const [active, setActive] = useState<string | null>(null);
  const total = shares.reduce((sum, s) => sum + s.value, 0);
  const shown = shares.filter((s) => s.value > 0);

  return (
    <figure className={cn("m-0 border border-border bg-card p-4", className)}>
      <figcaption className="flex items-baseline justify-between gap-2">
        <span className="text-sm font-bold">{title}</span>
        <span className="text-xs text-muted-foreground">{formatValue(total)}</span>
      </figcaption>

      {total === 0 ? (
        <p className="mt-3 text-sm text-muted-foreground">{empty}</p>
      ) : (
        <>
          <div className="mt-3 flex h-6 w-full gap-[2px]" role="img" aria-label={title}>
            {shown.map((s) => (
              <button
                key={s.key}
                type="button"
                title={`${s.label}: ${formatValue(s.value)}`}
                aria-label={`${s.label}: ${formatValue(s.value)}`}
                onPointerEnter={() => setActive(s.key)}
                onPointerLeave={() => setActive(null)}
                onFocus={() => setActive(s.key)}
                onBlur={() => setActive(null)}
                className="h-full min-w-[3px] rounded-[2px] outline-none transition-opacity focus-visible:ring-3 focus-visible:ring-ring/40"
                style={{
                  width: `${(s.value / total) * 100}%`,
                  background: s.color,
                  opacity: active === null || active === s.key ? 1 : 0.55,
                }}
              />
            ))}
          </div>

          <ul className="mt-3 flex flex-wrap gap-x-4 gap-y-1.5 text-xs">
            {shares.map((s) => (
              <li
                key={s.key}
                className="flex items-center gap-1.5"
                onPointerEnter={() => setActive(s.key)}
                onPointerLeave={() => setActive(null)}
              >
                <span
                  aria-hidden
                  className="size-2.5 shrink-0 rounded-[2px]"
                  style={{ background: s.color }}
                />
                <span className="text-muted-foreground">{s.label}</span>
                <span className="font-bold text-foreground">{formatValue(s.value)}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </figure>
  );
}
