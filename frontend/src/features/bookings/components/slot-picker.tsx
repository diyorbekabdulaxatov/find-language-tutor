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
  onPreview,
}: {
  slug: string;
  teacherTimezone: string;
  isTrial: boolean;
  onPick: (sel: SlotSelection) => void;
  /** fires as soon as a time is highlighted, so the summary can follow along */
  onPreview?: (sel: SlotSelection | null) => void;
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
          <span className="text-sm font-bold">{t("lessonLength")}</span>
          <div className="flex flex-wrap gap-2">
            {DURATION_OPTIONS.map((d) => (
              <button
                key={d}
                type="button"
                onClick={() => {
                  setDuration(d);
                  onPreview?.(null);
                }}
                className={cn(
                  "h-10 rounded-md border px-4 text-sm font-bold transition-colors",
                  duration === d
                    ? "border-foreground bg-ink text-ink-foreground"
                    : "border-border text-foreground hover:bg-accent",
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
          <span className="text-sm font-bold">{t("pickTime")}</span>
          <span className="text-xs text-muted-foreground">{t("shownInYourTime", { tz: viewerTz })}</span>
        </div>

        {loading && (
          <div className="mt-3 h-48 animate-pulse bg-muted" />
        )}

        {error && (
          <p className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}

        {!loading && !error && days.length === 0 && (
          <p className="mt-3 border border-border px-3 py-10 text-center text-sm text-muted-foreground">
            {t("noOpenTimes")}
          </p>
        )}

        {!loading && !error && currentDay && (
          <div className="mt-3 flex flex-col gap-4">
            <div role="tablist" className="flex gap-6 overflow-x-auto border-b border-border">
              {days.map((d) => (
                <button
                  key={d.key}
                  type="button"
                  role="tab"
                  aria-selected={currentDay.key === d.key}
                  onClick={() => {
                    setActiveDay(d.key);
                    setSelected(null);
                    onPreview?.(null);
                  }}
                  className={cn(
                    "-mb-px shrink-0 border-b-2 pb-2 text-sm font-bold whitespace-nowrap transition-colors",
                    currentDay.key === d.key
                      ? "border-foreground text-foreground"
                      : "border-transparent text-muted-foreground hover:text-foreground",
                  )}
                >
                  {d.label}
                  <span className="ml-1.5 text-xs font-normal text-muted-foreground">
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
                    onClick={() => {
                      setSelected(s);
                      onPreview?.({ slot: s, durationMinutes: duration, isTrial });
                    }}
                    className={cn(
                      "h-11 rounded-md border text-sm font-bold transition-colors",
                      on
                        ? "border-foreground bg-ink text-ink-foreground"
                        : "border-border text-foreground hover:bg-accent",
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
        <div className="flex flex-col gap-3 border border-border bg-card p-4 shadow-card">
          <div className="text-sm">
            <div className="font-bold">
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
          <button
            type="button"
            onClick={() =>
              onPick({ slot: selected, durationMinutes: duration, isTrial })
            }
            className="flex h-12 w-full items-center justify-center rounded-md bg-primary text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8]"
          >
            {t("continue")}
          </button>
        </div>
      )}
    </div>
  );
}
