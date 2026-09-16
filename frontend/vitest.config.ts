import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

// Unit tests for the pure logic (timezone math, formatting, locale
// negotiation, the auth fetch wrapper) and the message catalogues. Runs in
// node — nothing here renders React; the golden path is Playwright's job.
export default defineConfig({
  test: {
    include: ["src/**/*.test.ts"],
    environment: "node",
  },
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
});
