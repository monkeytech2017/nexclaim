import { defineConfig, devices } from '@playwright/test'

// Minimal smoke config. Chromium-only, headless, single baseURL.
// Tests intercept every /api/backend/* call via page.route() — no real backend
// or DB is required. The Next.js dev server is booted by webServer below
// and reused between runs when available.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: 'list',

  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
    // Seed a fake key in localStorage before each test that relies on an
    // authed session (see fixtures/auth.ts). Individual tests may override.
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  webServer: {
    command: 'npm run dev',
    port: 3000,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
  },
})
