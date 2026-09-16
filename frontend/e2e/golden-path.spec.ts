import { expect, test } from "@playwright/test";

/**
 * The one flow the product exists for: a new student signs up, finds a
 * teacher, books a lesson, pays (fake provider), and sees it confirmed.
 * Runs against the seeded backend; each run registers a fresh account so it
 * is repeatable without a reset.
 */
test("a new student can sign up, book a lesson and pay for it", async ({ page }) => {
  const email = `e2e-${Date.now()}@example.com`;

  // --- sign up ---
  await page.goto("/signup");
  await page.getByLabel("Name").fill("E2E Student");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: "Sign up" }).click();
  // Lands on the dashboard (the ?next= default) and the header shows the account.
  await expect(page).toHaveURL(/\/dashboard/);
  await expect(page.getByRole("button", { name: "Account menu" })).toBeVisible();

  // --- find a teacher ---
  await page.goto("/teachers");
  await expect(page.getByRole("heading", { name: "Find your teacher" })).toBeVisible();
  await page.getByRole("link", { name: /Nodira Karimova/ }).first().click();
  await expect(page.getByRole("heading", { name: /Nodira Karimova/ })).toBeVisible();

  // --- book ---
  await page.getByRole("link", { name: "Book a lesson", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Book a lesson" })).toBeVisible();

  // First open day, first slot. The seed teacher has weekly hours, so the
  // next two weeks always contain at least one.
  const slotButtons = page.locator("div.grid button");
  await expect(slotButtons.first()).toBeVisible();
  await slotButtons.first().click();
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByRole("heading", { name: "Confirm your lesson" })).toBeVisible();
  await expect(page.getByText("Nodira Karimova", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Continue to payment" }).click();

  // --- pay with the fake provider's succeeding card ---
  await expect(page.getByRole("heading", { name: "Payment" })).toBeVisible();
  await page.getByLabel(/Test card — succeeds/).check();
  await page.getByRole("button", { name: /^Pay / }).click();

  await expect(page.getByRole("heading", { name: "Lesson booked" })).toBeVisible();
  await expect(page.getByText(/Your lesson with Nodira Karimova is confirmed/)).toBeVisible();

  // --- it shows up in the student's bookings ---
  await page.getByRole("link", { name: "All bookings" }).click();
  await expect(page.getByRole("heading", { name: "Bookings" })).toBeVisible();
  const upcoming = page.getByRole("heading", { name: "Upcoming" });
  await expect(upcoming).toBeVisible();
  await expect(page.getByText("Confirmed").first()).toBeVisible();
});

test("the UI follows the language switcher", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Find a language teacher who gets you talking" })).toBeVisible();

  await page.getByRole("button", { name: "Change language" }).click();
  await page.getByRole("menuitem", { name: "Русский" }).click();

  await expect(page.getByRole("heading", { name: "Найдите преподавателя, с которым вы заговорите" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "ru");
});
