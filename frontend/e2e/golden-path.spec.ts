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
  // A new student lands on the teacher catalog, signed in.
  await expect(page).toHaveURL(/\/teachers$/);
  await expect(page.getByRole("button", { name: "Account menu" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Find your teacher" })).toBeVisible();
  await page.getByRole("link", { name: /Nodira Karimova/ }).first().click();
  await expect(page.getByRole("heading", { name: /Nodira Karimova/ })).toBeVisible();

  // --- book ---
  await page.getByRole("link", { name: "Book a lesson", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Book a lesson" })).toBeVisible();

  // First open day, first slot. The seed teacher has weekly hours, so the
  // next two weeks always contain at least one. A time button is named by the
  // time it holds ("11:00"), which the duration chips ("60 min") never match.
  const slotButtons = page.getByRole("button", { name: /^\d{1,2}:\d{2}$/ });
  await expect(slotButtons.first()).toBeVisible();
  await slotButtons.first().click();
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByRole("heading", { name: "Confirm your lesson" })).toBeVisible();
  await expect(page.getByText("Nodira Karimova", { exact: true }).first()).toBeVisible();
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
  await expect(page.getByRole("heading", { name: "Learn a language with a real teacher" })).toBeVisible();

  await page.getByRole("banner").getByRole("button", { name: "Change language" }).click();
  await page.getByRole("menuitem", { name: "Русский" }).click();

  await expect(page.getByRole("heading", { name: "Учите язык с настоящим преподавателем" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "ru");
});

test("signing up to teach lands on the profile form", async ({ page }) => {
  await page.goto("/signup?role=teacher");
  await expect(page.getByText(/signing up to teach/)).toBeVisible();
  await page.getByLabel("Name").fill("E2E Teacher");
  await page.getByLabel("Email").fill(`e2e-teacher-${Date.now()}@example.com`);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: "Sign up" }).click();
  await expect(page).toHaveURL(/\/dashboard/);
  // A fresh account is unverified: the wizard is there to fill in, but the
  // final submit waits for the email link (the backend would 403
  // `email_not_verified`). Walk the three steps to get to it.
  await expect(page.getByText("Confirm your email to submit a profile")).toBeVisible();
  await expect(page.getByLabel("Display name")).toHaveValue("E2E Teacher");
  await page.getByLabel("Headline").fill("Conversation practice");
  await page.getByLabel("City").fill("Tashkent");
  // "Next" without a taught language stays on step 1 with a hint.
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page.getByText("Add at least one language you teach.")).toBeVisible();
  await page.getByRole("button", { name: "Add language" }).click();
  await page.getByRole("button", { name: "Choose a language" }).click();
  await page.getByPlaceholder("Search languages…").fill("Engl");
  await page.getByRole("button", { name: /English/ }).click();
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page.getByText("Step 2 of 3")).toBeVisible();
  await page.getByLabel("Price per hour (so'm)").fill("90000");
  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page.getByText("Step 3 of 3")).toBeVisible();
  const submit = page.getByRole("button", { name: "Create profile" });
  await expect(submit).toBeVisible();
  await expect(submit).toBeDisabled();
});
