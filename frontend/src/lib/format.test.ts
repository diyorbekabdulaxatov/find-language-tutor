import { describe, expect, it } from "vitest";
import { formatCompact, formatMoney } from "./format";

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
