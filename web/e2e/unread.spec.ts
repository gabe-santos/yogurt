import { expect, test } from '@playwright/test';

import { password } from './env';

// Runs against the Feed the journeys before this one subscribed to. Unread Only
// narrows the Entry List; it does not rearrange it while the reader works. The
// list used to drop an Entry the moment reading it made it Read, so reading
// down a narrowed list pulled rows out from under the cursor at every step.
test('Unread Only narrows the list once and then holds it still', async ({
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
  await page.getByTestId('unread-only').click();
  const entries = page.getByTestId('entry');
  await expect(entries).toHaveCount(2);

  // Selecting the first marks it Read where it stands.
  await entries.first().click();
  await expect(page.getByTestId('reading-pane')).toBeVisible();
  await expect(entries).toHaveCount(2);
  await expect(entries.first()).not.toContainText('unread');

  // Moving on is what used to empty the list one arrival at a time. Both rows
  // stay, both now Read, and the reader keeps reading down a list that holds.
  await page.keyboard.press('j');
  await expect(entries).toHaveCount(2);
  await expect(entries.nth(1)).not.toContainText('unread');
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // The last row is the last row: j has nothing further to reach for.
  await page.keyboard.press('j');
  await expect(entries).toHaveCount(2);
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // Unread Only is the reader's own preference and survives the reload. So does
  // their place: the Entry they were reading is Read and outside the narrowed
  // selection, and is put back into the page the server built without it.
  await page.reload();
  await expect(page.getByTestId('unread-only')).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await expect(entries).toHaveCount(1);
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // Turning it off and on again is the rebuild the reader asked for, and now
  // that both Entries are Read there is nothing left to narrow to.
  await page.getByTestId('unread-only').click();
  await expect(entries).toHaveCount(2);
  await page.getByTestId('unread-only').click();
  await expect(entries).toHaveCount(0);
  await expect(page.getByText('Nothing unread here.')).toBeVisible();
  await expect(
    page.getByText('Turn Unread off to see everything in All Feeds.'),
  ).toBeVisible();

  // An emptied Collection still carries exactly one h1 — the Collection's
  // own name — and no h2, since no Entry is open. See issue #37.
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('h2')).toHaveCount(0);

  // Left off for the journeys that follow, which read an unnarrowed list.
  await page.getByTestId('unread-only').click();
  await expect(entries).toHaveCount(2);
});
