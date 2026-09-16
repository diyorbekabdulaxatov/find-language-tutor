import { describe, expect, it } from "vitest";
import { isLocale, negotiateLocale } from "./config";

describe("negotiateLocale", () => {
  it("takes the first supported primary subtag in order", () => {
    expect(negotiateLocale("ru-RU,ru;q=0.9,en;q=0.8")).toBe("ru");
    expect(negotiateLocale("uz-Latn-UZ,ru;q=0.8")).toBe("uz");
    expect(negotiateLocale("fr-FR,fr;q=0.9,uz;q=0.5")).toBe("uz");
    expect(negotiateLocale("en-GB,en;q=0.9")).toBe("en");
  });

  it("falls back to English for nothing supported or no header", () => {
    expect(negotiateLocale("fr,de")).toBe("en");
    expect(negotiateLocale(null)).toBe("en");
    expect(negotiateLocale("")).toBe("en");
  });

  it("is case-insensitive", () => {
    expect(negotiateLocale("UZ")).toBe("uz");
  });
});

describe("isLocale", () => {
  it("accepts only the three locales", () => {
    expect(isLocale("en")).toBe(true);
    expect(isLocale("uz")).toBe(true);
    expect(isLocale("fr")).toBe(false);
    expect(isLocale(undefined)).toBe(false);
  });
});
