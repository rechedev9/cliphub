import { defineConfig, devices } from '@playwright/test';

const BASE_URL = 'http://127.0.0.1:3200';

export default defineConfig({
  testDir: './e2e-hosted',
  outputDir: './test-results/hosted',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list']],
  timeout: 120_000,
  expect: { timeout: 20_000 },
  use: {
    baseURL: BASE_URL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'node scripts/hosted-e2e-server.mjs',
    url: BASE_URL,
    reuseExistingServer: false,
    timeout: 600_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
