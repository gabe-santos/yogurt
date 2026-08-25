import { expect, test } from '@playwright/test';

import { password } from './env';

// Runs after reading.spec.ts: the suite shares one database, and this
// journey reuses the Feed and Entries that journey already subscribed to
// rather than subscribing again, which the app would refuse.
test.use({ viewport: { width: 390, height: 844 } });

test('on a phone-sized viewport the sidebar is a sheet and reading still works', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // The Feed/Group sidebar starts as a sheet, not a fixed column: its
  // contents are absent until the trigger opens it.
  const sidebarSheet = page.getByRole('dialog', { name: 'Sidebar' });
  await expect(sidebarSheet).toBeHidden();
  await expect(page.getByLabel('Feed or site address')).toBeHidden();

  await page.getByRole('button', { name: 'Toggle Sidebar' }).click();
  await expect(sidebarSheet).toBeVisible();
  await expect(sidebarSheet.getByLabel('Feed or site address')).toBeVisible();
  await expect(
    sidebarSheet.getByRole('button', { name: 'All Feeds' }),
  ).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(sidebarSheet).toBeHidden();

  // The Entry list stays a single, full-width column rather than wrapping
  // into columns or overflowing the viewport.
  const scrollWidth = await page.evaluate(
    () => document.documentElement.scrollWidth,
  );
  expect(scrollWidth).toBeLessThanOrEqual(390);

  // Reading behaviour — open, next/previous, close — survives the narrow
  // viewport untouched.
  const entries = page.getByTestId('entry');
  await entries.first().click();
  const drawer = page.getByTestId('entry-drawer');
  await expect(drawer).toBeVisible();
  const firstContent = await page.getByTestId('entry-content').textContent();

  await page.keyboard.press('j');
  await expect(page.getByTestId('entry-content')).not.toHaveText(
    firstContent ?? '',
  );

  await page.keyboard.press('k');
  await expect(page.getByTestId('entry-content')).toHaveText(
    firstContent ?? '',
  );

  await page.keyboard.press('Escape');
  await expect(drawer).toBeHidden();
});
