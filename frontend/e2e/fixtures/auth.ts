import { test as base, expect, type Page, type Route, type Request } from '@playwright/test'

// All Next.js API calls go through /api/backend/* (see next.config.js rewrites
// and frontend/src/lib/api.ts line 3). We stub them so tests are hermetic —
// no Go backend or DB required.

const NOW = new Date().toISOString()

interface MockOptions {
  // When true, whoami responds 200 with an admin identity. When false, 401.
  // Default: true (fixture pre-writes the key before any navigation).
  authed?: boolean
}

export async function installApiMocks(page: Page, opts: MockOptions = {}): Promise<void> {
  const authed = opts.authed ?? true

  await page.route('**/api/backend/**', async (route: Route, request: Request) => {
    const url = new URL(request.url())
    const path = url.pathname.replace(/^.*\/api\/backend/, '')
    const hasAuth = (request.headers()['authorization'] ?? '').startsWith('Bearer ')

    // Health check — always OK
    if (path === '/healthz') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true, service: 'nexclaim' }),
      })
    }

    // whoami — 200 if Authorization header is set AND fixture is "authed",
    // otherwise 401. This lets the login test drive the form end-to-end while
    // the fixture pre-seeded path short-circuits through the guard.
    if (path === '/api/v1/auth/whoami') {
      if (hasAuth && authed) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ id: 'test-id', role: 'admin', name: 'test-admin' }),
        })
      }
      return route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'unauthorized' }),
      })
    }

    // Dashboard stats — 1 batch, 1 record, zero errors, sample chart data.
    if (path.startsWith('/api/v1/dashboard/stats')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          summary: {
            batches_total:     1,
            records_total:     1,
            records_errors:    0,
            ccodes_open:       0,
            avg_send_ms:       120,
            send_success_rate: 1,
          },
          by_format: [{ format: '16FILES', count: 1, records: 1 }],
          by_status: [{ status: 'sent', count: 1 }],
          by_day:    [{ day: '2026-04-23', formats: { '16FILES': 1 } }],
          top_ccodes: [],
        }),
      })
    }

    // Claim batches — one sample row so the submissions list renders a table.
    if (path.startsWith('/api/v1/claim/batches')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [{
            batch_id:      'B1',
            hcode:         '12345',
            period:        '202504',
            inscl:         'UCS',
            format:        '16FILES',
            sender:        'FDH',
            status:        'sent',
            total_records: 1,
            valid_records: 1,
            error_records: 0,
            created_at:    NOW,
            attempt_no:    1,
            c_code_count:  0,
            c_code_open:   0,
          }],
        }),
      })
    }

    // Hospital master — populates filter dropdowns on dashboard + submissions.
    if (path.startsWith('/api/v1/master/hospitals')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          hospitals: [{ hcode: '12345', name_th: 'Test Hospital', is_active: true }],
        }),
      })
    }

    // Anything else — log and return empty object so the app never hard-errors.
    // eslint-disable-next-line no-console
    console.warn(`[e2e mock] unmocked GET ${path} — returning {}`)
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: '{}',
    })
  })
}

// Pre-seed the API key in localStorage via addInitScript, so AuthGuard doesn't
// bounce us to /login on navigation to (dashboard)/* routes.
export async function seedApiKey(page: Page, key = 'nck_test_fixture_key'): Promise<void> {
  await page.addInitScript((k) => {
    try { window.localStorage.setItem('nexclaim.apiKey', k) } catch { /* ignore */ }
  }, key)
}

// Extended test with a `authedPage` fixture that:
//   1. installs the API mocks (authed=true),
//   2. seeds the API key before the first navigation.
export const test = base.extend<{ authedPage: Page }>({
  authedPage: async ({ page }, use) => {
    await installApiMocks(page, { authed: true })
    await seedApiKey(page)
    await use(page)
  },
})

export { expect }
