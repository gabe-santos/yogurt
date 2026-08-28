import { expect, test } from '@playwright/test';

import { password } from './env';

test('the reader creates, sees the last use of, and revokes a device token', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  // Device tokens live in the menu at the foot of the Collection List.
  await page.getByTestId('reader-menu').click();
  await page.getByRole('menuitem', { name: 'Device tokens' }).click();
  const dialog = page.getByTestId('device-tokens-dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText('No device tokens yet.')).toBeVisible();

  await dialog.getByLabel('Name').fill('desktop shell');
  await dialog.getByRole('button', { name: 'Create' }).click();

  // The raw value is shown once, at creation.
  const revealed = dialog.getByTestId('revealed-token');
  await expect(revealed).toBeVisible();
  const raw = await revealed.textContent();
  expect(raw).toBeTruthy();
  await dialog.getByRole('button', { name: 'Done' }).click();
  await expect(revealed).toBeHidden();

  await expect(dialog.getByText('desktop shell')).toBeVisible();
  await expect(dialog.getByText('Never used')).toBeVisible();

  // The token authenticates a request, and the dialog reflects that once
  // reopened.
  const authenticated = await page.evaluate(async (token) => {
    const response = await fetch('/api/feeds', {
      headers: { Authorization: `Bearer ${token}` },
    });
    return response.status;
  }, raw);
  expect(authenticated).toBe(200);

  await page.getByRole('button', { name: 'Close' }).first().click();
  await expect(dialog).toBeHidden();
  await page.getByTestId('reader-menu').click();
  await page.getByRole('menuitem', { name: 'Device tokens' }).click();
  await expect(dialog.getByText('desktop shell')).toBeVisible();
  await expect(dialog.getByText('Never used')).toBeHidden();

  // Revoking removes it from the list and stops it authenticating.
  await dialog.getByRole('button', { name: 'Revoke' }).click();
  await expect(dialog.getByText('desktop shell')).toBeHidden();
  await expect(dialog.getByText('No device tokens yet.')).toBeVisible();

  const revokedStatus = await page.evaluate(async (token) => {
    const response = await fetch('/api/feeds', {
      headers: { Authorization: `Bearer ${token}` },
    });
    return response.status;
  }, raw);
  expect(revokedStatus).toBe(401);
});

test('the footer menu blocks background shortcuts while open, and Escape closes only the menu', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  // No Feed subscribed: the suite shares one database across every spec
  // file (see subscribe.spec.ts), so this journey stays self-contained by
  // never touching it, and exercises the 'a' binding instead, which needs
  // no Feed to fire.
  const menu = page.getByTestId('reader-menu');
  const content = page.getByTestId('reader-menu-content');
  const addFeedDialog = page.getByTestId('add-feed-dialog');
  await menu.click();
  await expect(content).toBeVisible();

  // The menu is a bits-ui DropdownMenu: it already closes itself on Escape.
  // The app's own window-level shortcut handler must stay out of its way —
  // neither running the list's own shortcuts underneath it (a) nor running
  // its own "close whatever is open" fallback once the menu has already
  // closed itself (Escape).
  await page.keyboard.press('a');
  await expect(content).toBeVisible();
  await expect(addFeedDialog).toBeHidden();

  await page.keyboard.press('Escape');
  await expect(content).toBeHidden();
  await expect(menu).toBeFocused();
  await expect(addFeedDialog).toBeHidden();

  // With the menu closed, the same key reaches the app again.
  await page.keyboard.press('a');
  await expect(addFeedDialog).toBeVisible();
});
