import { expect, test } from '@playwright/test';

import { password } from './env';

// Runs against the Feed the journeys before this one subscribed to. Selecting
// an Entry is opening it, so in the Unread filter the Entry being read would
// otherwise fall out of its own list the moment it was selected — and the
// arrival that replaced it with it, one after another, until the list had
// emptied itself. It stays until the reader leaves it.
test('the Unread filter keeps the Entry being read and drops it when the reader moves on', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // Both Entries unread, whatever the journeys before this one left behind.
  await page.evaluate(async () => {
    const response = await fetch('/api/entries');
    const body = (await response.json()) as { entries: { id: number }[] };
    for (const entry of body.entries) {
      await fetch(`/api/entries/${entry.id}/state`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ read: false, starred: false, archived: false }),
      });
    }
  });
  await page.reload();
  await page.getByTestId('filter-unread').click();
  const entries = page.getByTestId('entry');
  await expect(entries).toHaveCount(2);

  // Selecting the first marks it Read without taking it out of the list.
  await entries.first().click();
  await expect(page.getByTestId('reading-pane')).toBeVisible();
  await expect(entries).toHaveCount(2);
  await expect(entries.first()).not.toContainText('unread');

  // Moving on drops the one left behind, and stops there: the arrival is Read
  // on the same terms, and does not take the rest of the list down with it.
  await page.keyboard.press('j');
  await expect(entries).toHaveCount(1);
  await expect(entries.first()).toContainText('Fire, and how to keep it');
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // The last row is the last row: j has nothing further to reach for.
  await page.keyboard.press('j');
  await expect(entries).toHaveCount(1);
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );
});
