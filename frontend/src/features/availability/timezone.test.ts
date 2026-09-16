import { describe, expect, it } from "vitest";
import { localToUtc, minutesToLabel, utcToLocal, type WeeklySlot } from "./timezone";

const h = (hours: number) => hours * 60;

describe("localToUtc / utcToLocal", () => {
  it("shifts Tashkent (UTC+5, no DST) hours back by five", () => {
    const local: WeeklySlot[] = [{ weekday: 1, startMinute: h(9), endMinute: h(17) }];
    expect(localToUtc(local, "Asia/Tashkent")).toEqual([
      { weekday: 1, startMinute: h(4), endMinute: h(12) },
    ]);
  });

  it("splits a slot that straddles UTC midnight into two", () => {
    // Mon 02:00–07:00 in Tashkent is Sun 21:00 → Mon 02:00 UTC.
    const local: WeeklySlot[] = [{ weekday: 1, startMinute: h(2), endMinute: h(7) }];
    expect(localToUtc(local, "Asia/Tashkent")).toEqual([
      { weekday: 0, startMinute: h(21), endMinute: 1440 },
      { weekday: 1, startMinute: 0, endMinute: h(2) },
    ]);
  });

  it("wraps Sunday early morning back to Saturday night", () => {
    const local: WeeklySlot[] = [{ weekday: 0, startMinute: h(1), endMinute: h(3) }];
    expect(localToUtc(local, "Asia/Tashkent")).toEqual([
      { weekday: 6, startMinute: h(20), endMinute: h(22) },
    ]);
  });

  it("round-trips through UTC and merges the split halves back", () => {
    const local: WeeklySlot[] = [
      { weekday: 1, startMinute: h(2), endMinute: h(7) },
      { weekday: 3, startMinute: h(10), endMinute: h(12) },
    ];
    const back = utcToLocal(localToUtc(local, "Asia/Tashkent"), "Asia/Tashkent");
    expect(back).toEqual(local);
  });

  it("is the identity for UTC", () => {
    const slots: WeeklySlot[] = [{ weekday: 5, startMinute: h(8), endMinute: h(20) }];
    expect(localToUtc(slots, "UTC")).toEqual(slots);
    expect(utcToLocal(slots, "UTC")).toEqual(slots);
  });

  it("treats a slot ending exactly at local midnight as ending at 1440, not the next day", () => {
    const local: WeeklySlot[] = [{ weekday: 2, startMinute: h(22), endMinute: 1440 }];
    const utc = localToUtc(local, "Asia/Tashkent");
    expect(utc).toEqual([{ weekday: 2, startMinute: h(17), endMinute: h(19) }]);
    expect(utcToLocal(utc, "Asia/Tashkent")).toEqual(local);
  });

  it("merges adjacent slots on the same day", () => {
    const local: WeeklySlot[] = [
      { weekday: 1, startMinute: h(9), endMinute: h(12) },
      { weekday: 1, startMinute: h(12), endMinute: h(15) },
    ];
    expect(localToUtc(local, "UTC")).toEqual([{ weekday: 1, startMinute: h(9), endMinute: h(15) }]);
  });

  it("handles a negative-offset zone (America/New_York) forward across midnight", () => {
    // NY is UTC-4 or UTC-5 depending on the reference week; either way
    // Mon 20:00–23:00 local lands on Tue 00:00–04:00 UTC ± an hour.
    const local: WeeklySlot[] = [{ weekday: 1, startMinute: h(20), endMinute: h(23) }];
    const utc = localToUtc(local, "America/New_York");
    expect(utc).toHaveLength(1);
    expect(utc[0].weekday).toBe(2);
    expect(utc[0].endMinute - utc[0].startMinute).toBe(h(3));
    expect(utcToLocal(utc, "America/New_York")).toEqual(local);
  });
});

describe("minutesToLabel", () => {
  it("zero-pads hours and minutes", () => {
    expect(minutesToLabel(0)).toBe("00:00");
    expect(minutesToLabel(h(9) + 5)).toBe("09:05");
    expect(minutesToLabel(h(23) + 59)).toBe("23:59");
  });
});
