"use client";

import { useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import {
  BookingError,
  DURATION_OPTIONS,
  getSlots,
  type BookableSlot,
  type Duration,
} from "@/features/bookings/api";
import {
  formatTime,
  groupByDay,
  viewerTimezone,
} from "@/features/bookings/datetime";
import { Button } from "@/components/ui/button";

const TRIAL_DURATION = 30;

export interface SlotSelection {
  slot: BookableSlot;
  durationMinutes: Duration;
  isTrial: boolean;
}

export function SlotPicker({
  slug,
  teacherTimezone,
  isTrial,
  onPick,
}: {
  slug: string;
  teacherTimezone: string;
  isTrial: boolean;
  onPick: (sel: SlotSelection) => void;
}) {
  const viewerTz = useMemo(() => viewerTimezone(), []);
  const t = useTranslations("bookings");
  const locale = useLocale();
  const [duration, setDuration] = useState<Duration>(
    isTrial ? TRIAL_DURATION : 60,
  );
  const [slots, setSlots] = useState<BookableSlot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeDay, setActiveDay] = useState<string | null>(null);
  const [selected, setSelected] = useState<BookableSlot | null>(null);

  useEffect(() => {
    let alive = true;
    async function load() {
      setLoading(true);
      setError(null);
      setSelected(null);
      try {
        const res = await getSlots(slug, { duration });
        if (alive) setSlots(res.slots);
      } catch (err) {
        if (alive) {
          setError(err instanceof BookingError ? err.message : t("couldNotLoadTimes"));
        }
      } finally {
        if (alive) setLoading(false);
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [slug, duration, t]);

  const days = useMemo(() => groupByDay(slots, viewerTz, locale), [slots, viewerTz, locale]);
  const currentDay =
    days.find((d) => d.key === activeDay) ?? days[0] ?? null;

  const sameZone = viewerTz === teacherTimezone;

  return (
    <div className="flex flex-col gap-6">
      {!isTrial && (
        <div className="flex flex-col gap-2">
          <span className="text-sm font-medium">{t("lessonLength")}</span>
          <div className="flex flex-wrap gap-2">
            {DURATION_OPTIONS.map((d) => (
              <button
                key={d}
                type="button"
                onClick={() => setDuration(d)}
                className={cn(
                  "rounded-lg border px-3 py-1.5 text-sm transition-colors",
                  duration === d
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-border hover:bg-muted",
                )}
              >
                {t("min", { count: d })}
              </button>
            ))}
          </div>
        </div>
      )}

      <div>
        <div className="flex items-baseline justify-between">
          <span className="text-sm font-medium">{t("pickTime")}</span>
          <span className="text-xs text-muted-foreground">{t("shownInYourTime", { tz: viewerTz })}</span>
        </div>

        {loading && (
          <div className="mt-3 h-48 animate-pulse rounded-xl bg-muted" />
        )}

        {error && (
          <p className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}

        {!loading && !error && days.length === 0 && (
          <p className="mt-3 rounded-lg bg-muted px-3 py-6 text-center text-sm text-muted-foreground">
            {t("noOpenTimes")}
          </p>
        )}

        {!loading && !error && currentDay && (
          <div className="mt-3 flex flex-col gap-4">
            <div className="flex gap-2 overflow-x-auto pb-1">
              {days.map((d) => (
                <button
                  key={d.key}
                  type="button"
                  onClick={() => {
                    setActiveDay(d.key);
                    setSelected(null);
                  }}
                  className={cn(
                    "shrink-0 rounded-lg border px-3 py-2 text-sm transition-colors",
                    currentDay.key === d.key
                      ? "border-primary bg-accent"
                      : "border-border hover:bg-muted",
                  )}
                >
                  <span className="font-medium">{d.label}</span>
                  <span className="ml-1.5 text-xs text-muted-foreground">
                    {d.slots.length}
                  </span>
                </button>
              ))}
            </div>

            <div className="grid grid-cols-3 gap-2 sm:grid-cols-4">
              {currentDay.slots.map((s) => {
                const on = selected?.startAt === s.startAt;
                return (
                  <button
                    key={s.startAt}
                    type="button"
                    onClick={() => setSelected(s)}
                    className={cn(
                      "rounded-lg border py-2 text-sm transition-colors",
                      on
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-border hover:bg-muted",
                    )}
                    title={
                      sameZone
                        ? undefined
                        : t("forTheTeacher", { time: formatTime(s.startAt, teacherTimezone, locale) })
                    }
                  >
                    {formatTime(s.startAt, viewerTz, locale)}
                  </button>
                );
              })}
            </div>
          </div>
        )}
      </div>

      {selected && (
        <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4">
          <div className="text-sm">
            <div className="font-medium">
              {t("yourTime", {
                start: formatTime(selected.startAt, viewerTz, locale),
                end: formatTime(selected.endAt, viewerTz, locale),
              })}
            </div>
            {!sameZone && (
              <div className="text-muted-foreground">
                {t("teacherTime", {
                  start: formatTime(selected.startAt, teacherTimezone, locale),
                  end: formatTime(selected.endAt, teacherTimezone, locale),
                  tz: teacherTimezone,
                })}
              </div>
            )}
          </div>
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">
              {isTrial ? t("trialLesson") : t("min", { count: duration })}
            </span>
            <span className="font-display text-lg">
              {formatMoney(selected.price, locale)}
            </span>
          </div>
          <Button
            onClick={() =>
              onPick({ slot: selected, durationMinutes: duration, isTrial })
            }
          >
            {t("continue")}
          </Button>
        </div>
      )}
    </div>
  );
}
