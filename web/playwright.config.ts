import { defineConfig, devices } from '@playwright/test';

/** Presentation-contract e2e. No orchestrator: empty/offline states are the design. */
const PORT = Number(process.env.E2E_PORT ?? 3100);
const BASE_URL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: './e2e',
  outputDir: './test-results',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  // Next dev compiles a route on first request; more workers than this just
  // queue behind the same compiler.
  workers: 4,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
  timeout: 90_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: BASE_URL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  // Production build: next dev's HMR never attaches under Playwright.
  webServer: {
    command: process.env.E2E_SKIP_BUILD === '1' ? 'pnpm run start' : 'pnpm run build && pnpm run start',
    url: BASE_URL,
    env: { PORT: String(PORT) },
    reuseExistingServer: true,
    timeout: 600_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
