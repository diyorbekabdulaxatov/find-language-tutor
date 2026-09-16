"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Clock } from "lucide-react";
import { hoursFromViewer, localTimeIn } from "@/lib/i18n";

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
  const t = useTranslations("profile");
  const locale = useLocale();
  const [time, setTime] = useState(initial);
  const [offsetHours, setOffsetHours] = useState<number | null>(null);

  useEffect(() => {
    const tick = () => {
      const now = new Date();
      setTime(localTimeIn(timezone, now, locale));
      setOffsetHours(hoursFromViewer(timezone, now));
    };
    tick();
    const id = setInterval(tick, 60_000);
    return () => clearInterval(id);
  }, [timezone, locale]);

  const offset =
    offsetHours === null
      ? ""
      : offsetHours === 0
        ? t("sameTime")
        : offsetHours > 0
          ? t("hoursAhead", { count: offsetHours })
          : t("hoursBehind", { count: -offsetHours });

  return (
    <span className="inline-flex items-center gap-1.5 text-muted-foreground">
      <Clock className="size-3.5" aria-hidden />
      <span>
        {t("localTime", { time, city })}
        {offset ? ` — ${offset}` : ""}
      </span>
    </span>
  );
}
