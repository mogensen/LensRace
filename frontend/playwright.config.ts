import { existsSync } from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { defineConfig, devices } from '@playwright/test'

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// require('dotenv').config();

// Claude Code sets CLAUDECODE=1 in every session (local or sandboxed) and,
// in its sandboxed remote environments, pre-installs a single Chromium
// build outside Playwright's own managed browser cache — at
// PLAYWRIGHT_BROWSERS_PATH, not the revision-pinned path `npx playwright
// install` would otherwise fetch, which those sandboxes have no egress to
// download anyway. Point straight at that binary when it's actually there;
// everywhere else (a human's machine, real CI) this does nothing and
// Playwright resolves the browser the normal way. See CLAUDE.md.
const claudeChromiumPath = process.env.PLAYWRIGHT_BROWSERS_PATH
  ? path.join(process.env.PLAYWRIGHT_BROWSERS_PATH, 'chromium')
  : undefined
const useClaudeChromium =
  !!process.env.CLAUDECODE && !!claudeChromiumPath && existsSync(claudeChromiumPath)

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: './e2e',
  /* Maximum time one test can run for. */
  timeout: 30 * 1000,
  expect: {
    /**
     * Maximum time expect() should wait for the condition to be met.
     * For example in `await expect(locator).toHaveText();`
     */
    timeout: 5000,
  },
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. Locally, cap well below Playwright's
   * default (CPU count) — several tests open two browser contexts each
   * (host+guest) against one shared Go+SQLite backend, and running 3
   * browser engines at full worker parallelism reliably starved that
   * backend under real load (ECONNREFUSED / timeouts unrelated to any
   * specific test, reproducible across unrelated pre-existing tests). */
  workers: process.env.CI ? 1 : 2,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: 'html',
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Maximum time each action such as `click()` can take. Defaults to 0 (no limit). */
    actionTimeout: 0,
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: process.env.CI ? 'http://localhost:4173' : 'http://localhost:5173',

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',

    /* CI has no display; Claude Code sandboxes don't either. */
    headless: !!process.env.CI || !!process.env.CLAUDECODE,
  },

  /* Configure projects for major browsers. Claude Code's sandboxes only
   * have Chromium available (see useClaudeChromium above) — Firefox and
   * WebKit aren't installed there, so skip them rather than fail every run;
   * a human running the full cross-browser suite still gets all three. */
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        ...(useClaudeChromium ? { launchOptions: { executablePath: claudeChromiumPath } } : {}),
      },
    },
    ...(process.env.CLAUDECODE
      ? []
      : [
          {
            name: 'firefox',
            use: {
              ...devices['Desktop Firefox'],
            },
          },
          {
            name: 'webkit',
            use: {
              ...devices['Desktop Safari'],
            },
          },
        ]),

    /* Test against mobile viewports. */
    // {
    //   name: 'Mobile Chrome',
    //   use: {
    //     ...devices['Pixel 5'],
    //   },
    // },
    // {
    //   name: 'Mobile Safari',
    //   use: {
    //     ...devices['iPhone 12'],
    //   },
    // },

    /* Test against branded browsers. */
    // {
    //   name: 'Microsoft Edge',
    //   use: {
    //     channel: 'msedge',
    //   },
    // },
    // {
    //   name: 'Google Chrome',
    //   use: {
    //     channel: 'chrome',
    //   },
    // },
  ],

  /* Folder for test artifacts such as screenshots, videos, traces, etc. */
  // outputDir: 'test-results/',

  /* Run the backend API and the local frontend server before starting the tests. */
  webServer: [
    {
      // The Go backend the frontend's /api proxy talks to. Uses a
      // dedicated file-backed DB (not :memory:) because the connection
      // pool can open more than one connection under real concurrent
      // HTTP load (e.g. an open SSE stream alongside other requests),
      // and SQLite :memory: databases are per-connection unless using a
      // shared-cache URI.
      //
      // Builds a binary and runs that directly rather than `go run .`:
      // `go run` spawns the actual server as a *child* process with its
      // own PID, so killing/reusing based on the `go run` process doesn't
      // reliably reflect (or clean up) the real listener, which caused
      // orphaned/duplicate backends fighting over the same SQLite file.
      command:
        'go build -o /tmp/lensrace-playwright-e2e-server . && /tmp/lensrace-playwright-e2e-server',
      cwd: '..',
      env: { DB_PATH: '/tmp/lensrace-playwright-e2e.db', PORT: '3000' },
      port: 3000,
      reuseExistingServer: !process.env.CI,
      timeout: 30 * 1000,
    },
    {
      /**
       * Use the dev server by default for faster feedback loop.
       * Use the preview server on CI for more realistic testing.
       * Playwright will re-use the local server if there is already a dev-server running.
       */
      command: process.env.CI ? 'npm run preview' : 'npm run dev',
      port: process.env.CI ? 4173 : 5173,
      reuseExistingServer: !process.env.CI,
    },
  ],
})
