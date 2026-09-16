import { describe, expect, it } from "vitest";
import { formatDayLabel, formatFull, formatTime, groupByDay } from "./datetime";
import type { BookableSlot } from "./api";

const iso = "2026-02-03T09:30:00Z"; // Tue 3 Feb 2026, 09:30 UTC = 14:30 Tashkent

describe("format helpers", () => {
  it("renders the instant in the given zone, British style for English", () => {
    expect(formatTime(iso, "Asia/Tashkent")).toBe("14:30");
    expect(formatTime(iso, "UTC")).toBe("09:30");
    expect(formatDayLabel(iso, "Asia/Tashkent")).toBe("Tue 3 Feb");
    expect(formatFull(iso, "Asia/Tashkent")).toMatch(/^Tuesday,? 3 February 2026,? (at )?14:30$/);
  });

  it("uses the locale's names for ru and uz", () => {
    expect(formatDayLabel(iso, "Asia/Tashkent", "ru")).toMatch(/вт/);
    expect(formatDayLabel(iso, "Asia/Tashkent", "ru")).toMatch(/февр?\.?/);
    expect(formatFull(iso, "Asia/Tashkent", "uz")).toMatch(/fevral/);
  });
});

describe("groupByDay", () => {
  const slot = (start: string): BookableSlot => ({
    startAt: start,
    endAt: start,
    price: { amountMinor: 0, currency: "UZS" },
  });

  it("groups by the viewer's calendar day, not UTC's", () => {
    // 21:00 UTC Mon = 02:00 Tue in Tashkent.
    const days = groupByDay(
      [slot("2026-02-02T21:00:00Z"), slot("2026-02-02T10:00:00Z"), slot("2026-02-03T05:00:00Z")],
      "Asia/Tashkent",
    );
    expect(days.map((d) => d.key)).toEqual(["2026-02-02", "2026-02-03"]);
    expect(days[0].slots).toHaveLength(1);
    expect(days[1].slots).toHaveLength(2);
    expect(days[1].label).toBe("Tue 3 Feb");
  });

  it("returns days in chronological order regardless of input order", () => {
    const days = groupByDay([slot("2026-02-05T08:00:00Z"), slot("2026-02-01T08:00:00Z")], "UTC");
    expect(days.map((d) => d.key)).toEqual(["2026-02-01", "2026-02-05"]);
  });
});
