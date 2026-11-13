import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "@playwright/test";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendDir = path.resolve(dirname);
const backendDir = path.resolve(dirname, "..", "backend");

export default defineConfig({
  testDir: "./tests/e2e",
  outputDir: "./test-results",
  globalSetup: "./tests/e2e/global-setup.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 90_000,
  expect: { timeout: 15_000 },
  reporter: [["list"], ["html", { open: "never", outputFolder: "playwright-report" }]],
  use: {
    baseURL: process.env.E2E_APP_ORIGIN ?? "http://localhost:3000",
    browserName: "chromium",
    headless: true,
    actionTimeout: 20_000,
    navigationTimeout: 45_000,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: [
    {
      command: "sh -c 'go run ./cmd/migrate && exec go run ./cmd/api'",
      cwd: backendDir,
      url: "http://localhost:8080/healthz",
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
      env: {
        APP_ENV: "development",
        HTTP_ADDR: ":8080",
        DATABASE_URL: process.env.E2E_DATABASE_URL ?? "postgres://user:password@localhost:5432/school_db_e2e?sslmode=disable",
        REDIS_URL: process.env.E2E_REDIS_URL ?? "redis://localhost:6379",
        OCR_URL: process.env.E2E_OCR_URL ?? "http://localhost:8001",
        JWT_SECRET: process.env.E2E_JWT_SECRET ?? "e2e-local-secret-key-0123456789abcdef",
        CORS_ORIGIN: process.env.E2E_APP_ORIGIN ?? "http://localhost:3000",
      },
    },
    {
      command: "pnpm exec next start -H localhost -p 3000",
      cwd: frontendDir,
      url: "http://localhost:3000/login",
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
  ],
});
