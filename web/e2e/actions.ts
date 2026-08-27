// Reusable journey steps shared across the browser-seam specs.
import type { Page } from '@playwright/test';

// subscribeToFeed opens the Add Feed dialog, fills the address (and,
// optionally, a name), and submits — the one way any journey adds a Feed.
export async function subscribeToFeed(page: Page, url: string, name?: string) {
  await page.getByRole('button', { name: 'Add Feed' }).click();
  const dialog = page.getByTestId('add-feed-dialog');
  await dialog.getByLabel('Feed or site address').fill(url);
  if (name !== undefined) {
    await dialog.getByLabel('Name').fill(name);
  }
  await dialog.getByRole('button', { name: 'Subscribe' }).click();
}
