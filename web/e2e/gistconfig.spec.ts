import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';

import { password } from './env';

// clearAIConfig clears the Gist configuration directly, so the suite's
// shared database starts (and ends) unconfigured even if an assertion
// above fails mid-test.
async function clearAIConfig(page: Page) {
  return page.evaluate(() => fetch('/api/ai', { method: 'DELETE' }));
}

test('the reader configures Gists in the Yogurt Menu, the key stays hidden, and clearing turns them off', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  try {
    // Gists are configured in the Yogurt Menu at the foot of the Collection List.
    await page.getByTestId('yogurt-menu').click();
    await page.getByRole('menuitem', { name: 'Configure Gists' }).click();
    const dialog = page.getByTestId('gist-config-dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('No API key is stored.')).toBeVisible();

    await dialog
      .getByLabel('Endpoint URL')
      .fill('http://localhost:4272/v1/chat/completions');
    await dialog.getByLabel('Model').fill('test-model');
    await dialog.getByLabel('API key').fill('sk-secret');
    await dialog.getByRole('button', { name: 'Save' }).click();

    // The key is never shown back, only that one is stored; the field clears.
    await expect(dialog.getByText('An API key is stored.')).toBeVisible();
    await expect(dialog.getByLabel('API key')).toHaveValue('');

    // Reopening shows the saved url and model persisted, still with no key.
    await page.getByRole('button', { name: 'Close' }).first().click();
    await expect(dialog).toBeHidden();
    await page.getByTestId('yogurt-menu').click();
    await page.getByRole('menuitem', { name: 'Configure Gists' }).click();
    await expect(dialog.getByLabel('Endpoint URL')).toHaveValue(
      'http://localhost:4272/v1/chat/completions',
    );
    await expect(dialog.getByLabel('Model')).toHaveValue('test-model');
    await expect(dialog.getByText('An API key is stored.')).toBeVisible();
    await expect(dialog.getByLabel('API key')).toHaveValue('');

    // Clearing turns Gists off entirely, leaving the Instance unconfigured.
    await dialog.getByRole('button', { name: 'Turn off Gists' }).click();
    await expect(dialog.getByText('No API key is stored.')).toBeVisible();
    await expect(dialog.getByLabel('Endpoint URL')).toHaveValue('');
    await expect(dialog.getByLabel('Model')).toHaveValue('');

    await page.getByRole('button', { name: 'Close' }).first().click();
  } finally {
    await clearAIConfig(page);
  }
});
