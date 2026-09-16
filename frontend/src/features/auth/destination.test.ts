import { describe, expect, it } from "vitest";
import { postAuthDestination, safeNext } from "./destination";

describe("safeNext", () => {
  it("accepts only same-site absolute paths", () => {
    expect(safeNext("/bookings/1")).toBe("/bookings/1");
    expect(safeNext("/teachers?lang=en")).toBe("/teachers?lang=en");
    expect(safeNext("//evil.example")).toBeNull();
    expect(safeNext("/\\evil.example")).toBeNull();
    expect(safeNext("https://evil.example")).toBeNull();
    expect(safeNext("")).toBeNull();
    expect(safeNext(null)).toBeNull();
  });
});

describe("postAuthDestination", () => {
  it("honours next over everything", () => {
    expect(postAuthDestination({ mode: "signup", role: "teacher", next: "/bookings/x" })).toBe("/bookings/x");
    expect(postAuthDestination({ mode: "login", next: "/admin" })).toBe("/admin");
  });

  it("sends a new teacher to the dashboard and a new student to the catalog", () => {
    expect(postAuthDestination({ mode: "signup", role: "teacher" })).toBe("/dashboard");
    expect(postAuthDestination({ mode: "signup" })).toBe("/teachers");
    expect(postAuthDestination({ mode: "signup", role: "student" })).toBe("/teachers");
  });

  it("sends a returning user home", () => {
    expect(postAuthDestination({ mode: "login" })).toBe("/");
    expect(postAuthDestination({ mode: "login", role: "teacher" })).toBe("/");
  });
});
