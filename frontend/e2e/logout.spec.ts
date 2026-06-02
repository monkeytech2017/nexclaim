import { test, expect } from './fixtures/auth'

// Smoke test 3:
//   authed → /dashboard → click the LogOut icon button in Header →
//   redirect to /login AND localStorage no longer has `nexclaim.apiKey`.
test('logout clears stored key and redirects to /login', async ({ authedPage }) => {
  await authedPage.goto('/dashboard')

  // Sanity: key is present pre-logout.
  const before = await authedPage.evaluate(() => window.localStorage.getItem('nexclaim.apiKey'))
  expect(before).not.toBeNull()

  // Header renders identity chip + logout button once whoami resolves. The
  // button is labelled with aria-label "ออกจากระบบ" (see Header.tsx).
  const logoutBtn = authedPage.getByRole('button', { name: 'ออกจากระบบ' })
  await expect(logoutBtn).toBeVisible()
  await logoutBtn.click()

  await authedPage.waitForURL('**/login')

  const after = await authedPage.evaluate(() => window.localStorage.getItem('nexclaim.apiKey'))
  expect(after).toBeNull()
})
