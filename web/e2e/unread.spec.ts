import { expect, test } from '@playwright/test';

import { password } from './env';

// Runs against the Feed the journeys before this one subscribed to. Unread is a
// Collection of its own, and it does not rearrange itself while the reader
// works. It used to drop an Entry the moment reading it made it Read, so
// reading down it pulled rows out from under the cursor at every step.
test('Unread holds every unread Entry and then holds still', async ({
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

  // The unread count labels Unread, the Collection that holds exactly those
  // Entries, and not All Feeds.
  const badge = (collection: string) =>
    page
      .getByTestId(collection)
      .locator('xpath=..')
      .locator('[data-sidebar="menu-badge"]');
  await expect(badge('collection-unread')).toHaveText('2');
  await expect(badge('collection-all')).toHaveCount(0);

  await page.getByTestId('collection-unread').click();
  await expect(page.getByTestId('collection')).toHaveText('Unread');
  const entries = page.getByTestId('entry');
  await expect(entries).toHaveCount(2);

  // Selecting the first marks it Read where it stands.
  await entries.first().click();
  await expect(page.getByTestId('reading-pane')).toBeVisible();
  await expect(entries).toHaveCount(2);
  await expect(entries.first()).not.toContainText('unread');
  await expect(badge('collection-unread')).toHaveText('1');

  // Moving on is what used to empty the list one arrival at a time. Both rows
  // stay, both now Read, and the reader keeps reading down a list that holds.
  await page.keyboard.press('j');
  await expect(entries).toHaveCount(2);
  await expect(entries.nth(1)).not.toContainText('unread');
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );
  await expect(badge('collection-unread')).toHaveCount(0);

  // The last row is the last row: j has nothing further to reach for.
  await page.keyboard.press('j');
  await expect(entries).toHaveCount(2);
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // A reload opens on All Feeds, as it always has, with the list starting at
  // the Entry the reader was on.
  await page.reload();
  await expect(page.getByTestId('collection')).toHaveText('All Feeds');
  await expect(entries).toHaveCount(1);
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // Choosing Unread is the rebuild the reader asks for, and now that both
  // Entries are Read there is nothing left in it.
  await page.getByTestId('collection-unread').click();
  await expect(entries).toHaveCount(0);
  await expect(page.getByText('Nothing unread.')).toBeVisible();
  await expect(
    page.getByText('Every Entry from every Feed is Read.'),
  ).toBeVisible();

  // An emptied Collection still carries exactly one h1 — the Collection's
  // own name — and no h2, since no Entry is open. See issue #37.
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('h2')).toHaveCount(0);

  // Left on All Feeds for the journeys that follow.
  await page.getByTestId('collection-all').click();
  await expect(entries).toHaveCount(2);
});
