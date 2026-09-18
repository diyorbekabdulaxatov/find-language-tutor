import { describe, expect, it } from "vitest";
import { courseLengthParts, formatCompact, formatLectureLength, formatMoney } from "./format";

describe("formatMoney", () => {
  it("renders UZS as whole so'm with the locale's grouping and suffix", () => {
    expect(formatMoney({ amountMinor: 12_000_000, currency: "UZS" })).toBe("120,000 so'm");
    // ru and uz group with a (narrow) no-break space; assert on digits + suffix
    // rather than the exact whitespace codepoint.
    expect(formatMoney({ amountMinor: 12_000_000, currency: "UZS" }, "ru")).toMatch(/^120.000 сум$/);
    expect(formatMoney({ amountMinor: 12_000_000, currency: "UZS" }, "uz")).toMatch(/^120.000 so'm$/);
  });

  it("never shows tiyin", () => {
    expect(formatMoney({ amountMinor: 12_000_050, currency: "UZS" })).toBe("120,001 so'm");
  });

  it("renders USD with cents only when there are cents", () => {
    expect(formatMoney({ amountMinor: 1200, currency: "USD" })).toBe("$12");
    expect(formatMoney({ amountMinor: 1250, currency: "USD" })).toBe("$12.50");
  });

  it("falls back to so'm for an unknown locale", () => {
    expect(formatMoney({ amountMinor: 100, currency: "UZS" }, "de")).toMatch(/so'm$/);
  });
});

describe("formatCompact", () => {
  it("compacts thousands", () => {
    expect(formatCompact(1200)).toBe("1.2K");
    expect(formatCompact(999)).toBe("999");
  });
});

describe("formatLectureLength", () => {
  it("renders mm:ss below an hour", () => {
    expect(formatLectureLength(252)).toBe("4:12");
    expect(formatLectureLength(59)).toBe("0:59");
    expect(formatLectureLength(60)).toBe("1:00");
  });

  it("renders h:mm:ss at or above an hour", () => {
    expect(formatLectureLength(3750)).toBe("1:02:30");
    expect(formatLectureLength(3600)).toBe("1:00:00");
  });

  it("treats 0 and nonsense as unknown", () => {
    expect(formatLectureLength(0)).toBeNull();
    expect(formatLectureLength(-5)).toBeNull();
    expect(formatLectureLength(Number.NaN)).toBeNull();
  });
});

describe("courseLengthParts", () => {
  it("splits into hours and minutes", () => {
    expect(courseLengthParts(12000)).toEqual({ hours: 3, minutes: 20 });
    expect(courseLengthParts(2700)).toEqual({ hours: 0, minutes: 45 });
  });

  it("never rounds a real duration down to zero", () => {
    expect(courseLengthParts(20)).toEqual({ hours: 0, minutes: 1 });
  });

  it("treats 0 as unknown", () => {
    expect(courseLengthParts(0)).toBeNull();
  });
});
