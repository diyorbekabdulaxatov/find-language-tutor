"use client";

import { useEffect, useState } from "react";
import { Clock } from "lucide-react";
import { localTimeIn, offsetFromViewer } from "@/lib/i18n";

/**
 * Shows the teacher's current wall-clock time, ticking every minute.
 *
 * Why a client component: the server renders one fixed instant (`initial`), which
 * React uses for the first client render too — so hydration matches. The effect
 * then takes over with the live time and the viewer-relative offset (which needs
 * the browser's timezone).
 */
export function LocalTime({
  timezone,
  city,
  initial,
}: {
  timezone: string;
  city: string;
  /** server-rendered time string, keeps first paint stable */
  initial: string;
}) {
  const [time, setTime] = useState(initial);
  const [offset, setOffset] = useState<string | null>(null);

  useEffect(() => {
    const tick = () => {
      const now = new Date();
      setTime(localTimeIn(timezone, now));
      setOffset(offsetFromViewer(timezone, now));
    };
    tick();
    const id = setInterval(tick, 60_000);
    return () => clearInterval(id);
  }, [timezone]);

  return (
    <span className="inline-flex items-center gap-1.5 text-muted-foreground">
      <Clock className="size-3.5" aria-hidden />
      <span>
        {time} in {city}
        {offset ? ` — ${offset}` : ""}
      </span>
    </span>
  );
}
