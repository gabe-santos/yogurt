import { expect, test } from '@playwright/test';

import { password } from './env';

test('the reader creates, sees the last use of, and revokes a device token', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  await page.getByRole('button', { name: 'Device tokens' }).click();
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
  await page.getByRole('button', { name: 'Device tokens' }).click();
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
