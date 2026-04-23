import { test, expect } from '@playwright/test'
import { installApiMocks } from './fixtures/auth'

// Smoke test 1:
//   visit /login → paste an API key → submit → land on /dashboard →
//   all 4 stat cards render.
//
// Stat card labels live in src/app/(dashboard)/dashboard/page.tsx:
//   "Batches ทั้งหมด", "Records ส่งสำเร็จ", "C-code ค้าง", "Avg send latency"
test('login flow lands on dashboard with 4 stat cards', async ({ page }) => {
  await installApiMocks(page, { authed: true })

  await page.goto('/login')

  // Login page's on-mount whoami will 401 (no key yet) → form is shown.
  const input = page.locator('#api-key')
  await expect(input).toBeVisible()

  await input.fill('nck_smoke_test_key')
  await page.getByRole('button', { name: /เข้าสู่ระบบ/ }).click()

  await page.waitForURL('**/dashboard')

  // Assert all 4 KPI stat cards render.
  await expect(page.getByText('Batches ทั้งหมด')).toBeVisible()
  await expect(page.getByText('Records ส่งสำเร็จ')).toBeVisible()
  await expect(page.getByText('C-code ค้าง')).toBeVisible()
  await expect(page.getByText('Avg send latency')).toBeVisible()
})
