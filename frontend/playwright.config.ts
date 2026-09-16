import { defineConfig, devices } from "@playwright/test";

/**
 * End-to-end golden path against the real stack. Expects the backend on
 * :8080 with the seed loaded (`cd backend && make db-up migrate-up seed run`);
 * the frontend dev server is started here if it isn't already up.
 */
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  // A cold Turbopack compile of a route can take a few seconds on first
  // load; the default 5s expect timeout turned that into flakes.
  expect: { timeout: 15_000 },
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    locale: "en-GB",
    timezoneId: "Asia/Tashkent",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "npm run dev",
    url: "http://localhost:3000",
    reuseExistingServer: true,
    timeout: 60_000,
  },
});
