import { test, expect } from './fixtures/auth'

// Smoke test 2:
//   authed session (via fixture) → visit /submissions → page loads,
//   filter row is present (refresh button), and at least one table row
//   renders from the mocked claim_batch response.
test('submissions page renders filter + at least one row', async ({ authedPage }) => {
  await authedPage.goto('/submissions')

  // Filter row: the refresh button is the most stable anchor.
  const refresh = authedPage.getByRole('button', { name: /รีเฟรช/ })
  await expect(refresh).toBeVisible()

  // Mock returns 1 row (batch B1). The row count indicator renders "N รายการ".
  await expect(authedPage.getByText(/1 รายการ/)).toBeVisible()

  // Confirm the table row with our sample batch is visible — its TxnID
  // column is empty so anchor on HCODE / period.
  await expect(authedPage.getByText('12345 / 202504')).toBeVisible()
})
