"use client";

import { useEffect, useMemo, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import {
  AvailabilityError,
  getAvailability,
  replaceAvailability,
} from "@/features/availability/api";
import {
  localToUtc,
  minutesToLabel,
  utcToLocal,
  type WeeklySlot,
} from "@/features/availability/timezone";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const DAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const STEP = 30;
const DAY_MIN = 1440;
/** 00:00 … 24:00 in STEP-minute increments. */
const TIME_OPTIONS = Array.from({ length: DAY_MIN / STEP + 1 }, (_, i) => i * STEP);

interface Range {
  start: number;
  end: number;
}

type Week = Range[][]; // index 0..6

function emptyWeek(): Week {
  return Array.from({ length: 7 }, () => []);
}

function slotsToWeek(slots: WeeklySlot[]): Week {
  const week = emptyWeek();
  for (const s of slots) {
    week[s.weekday].push({ start: s.startMinute, end: s.endMinute });
  }
  for (const day of week) day.sort((a, b) => a.start - b.start);
  return week;
}

function weekToSlots(week: Week): WeeklySlot[] {
  const out: WeeklySlot[] = [];
  week.forEach((ranges, weekday) => {
    for (const r of ranges) {
      if (r.end > r.start) {
        out.push({ weekday, startMinute: r.start, endMinute: r.end });
      }
    }
  });
  return out;
}

export function AvailabilityEditor({ slug }: { slug: string }) {
  const [tz, setTz] = useState<string>("");
  const [week, setWeek] = useState<Week>(emptyWeek());
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    getAvailability(slug)
      .then((a) => {
        if (!alive) return;
        setTz(a.timezone);
        setWeek(slotsToWeek(utcToLocal(a.slots, a.timezone)));
      })
      .catch((err) => {
        if (!alive) return;
        setError(
          err instanceof AvailabilityError
            ? err.message
            : "Could not load your availability.",
        );
      })
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [slug]);

  const overlaps = useMemo(() => hasOverlap(week), [week]);

  function mutateDay(day: number, ranges: Range[]) {
    setWeek((w) => w.map((d, i) => (i === day ? ranges : d)));
    setStatus("idle");
  }

  async function handleSave() {
    setError(null);
    setStatus("saving");
    try {
      const utc = localToUtc(weekToSlots(week), tz);
      const saved = await replaceAvailability(slug, utc);
      setWeek(slotsToWeek(utcToLocal(saved.slots, tz)));
      setStatus("saved");
    } catch (err) {
      setStatus("idle");
      setError(
        err instanceof AvailabilityError
          ? err.message
          : "Could not save your availability.",
      );
    }
  }

  if (loading) {
    return <div className="h-64 animate-pulse rounded-xl bg-muted" />;
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">
        Recurring weekly hours, shown in your timezone
        {tz && <span className="font-medium text-foreground"> ({tz})</span>}.
        Students see these converted to their own time when booking.
      </p>

      <div className="flex flex-col divide-y divide-border rounded-2xl border border-border">
        {DAYS.map((label, day) => (
          <DayRow
            key={day}
            label={label}
            ranges={week[day]}
            onChange={(ranges) => mutateDay(day, ranges)}
          />
        ))}
      </div>

      {overlaps && (
        <p className="text-sm text-destructive">
          Some ranges on the same day overlap — fix those before saving.
        </p>
      )}
      {error && (
        <p
          role="alert"
          className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
        >
          {error}
        </p>
      )}

      <div className="flex items-center gap-3">
        <Button onClick={handleSave} disabled={status === "saving" || overlaps}>
          {status === "saving" ? "Saving…" : "Save availability"}
        </Button>
        {status === "saved" && (
          <span className="text-sm text-muted-foreground">Saved.</span>
        )}
      </div>
    </div>
  );
}

function DayRow({
  label,
  ranges,
  onChange,
}: {
  label: string;
  ranges: Range[];
  onChange: (ranges: Range[]) => void;
}) {
  return (
    <div className="flex flex-col gap-2 p-4 sm:flex-row sm:items-start sm:gap-4">
      <div className="w-28 shrink-0 pt-1.5 text-sm font-medium">{label}</div>
      <div className="flex flex-1 flex-col gap-2">
        {ranges.length === 0 && (
          <span className="pt-1.5 text-sm text-muted-foreground">Unavailable</span>
        )}
        {ranges.map((r, i) => (
          <div key={i} className="flex items-center gap-2">
            <TimeSelect
              value={r.start}
              onChange={(v) =>
                onChange(ranges.map((x, idx) => (idx === i ? { ...x, start: v } : x)))
              }
            />
            <span className="text-muted-foreground">–</span>
            <TimeSelect
              value={r.end}
              onChange={(v) =>
                onChange(ranges.map((x, idx) => (idx === i ? { ...x, end: v } : x)))
              }
            />
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label="Remove range"
              onClick={() => onChange(ranges.filter((_, idx) => idx !== i))}
            >
              <Trash2 />
            </Button>
          </div>
        ))}
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="self-start"
          onClick={() =>
            onChange([...ranges, { start: 9 * 60, end: 17 * 60 }])
          }
        >
          <Plus /> Add hours
        </Button>
      </div>
    </div>
  );
}

function TimeSelect({
  value,
  onChange,
}: {
  value: number;
  onChange: (v: number) => void;
}) {
  return (
    <Select value={String(value)} onValueChange={(v) => onChange(Number(v))}>
      <SelectTrigger className="w-[110px]">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {TIME_OPTIONS.map((m) => (
          <SelectItem key={m} value={String(m)}>
            {m === DAY_MIN ? "24:00" : minutesToLabel(m)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function hasOverlap(week: Week): boolean {
  return week.some((day) => {
    const sorted = [...day].sort((a, b) => a.start - b.start);
    return sorted.some(
      (r, i) => i > 0 && r.start < sorted[i - 1].end,
    );
  });
}
