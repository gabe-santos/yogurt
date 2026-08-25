import { expect, test } from '@playwright/test';

import { password, publisherURL } from './env';

test('the reader opens an Entry, reads it, and triages by keyboard', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByLabel('Feed or site address')).toBeVisible();

  await page.getByLabel('Feed or site address').fill(publisherURL);
  await page.getByRole('button', { name: 'Subscribe' }).click();
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // The Feed Icon slot renders the publisher's real, stored icon —
  // discovered at subscribe time — rather than the monogram fallback, and
  // stays decorative to assistive technology.
  await expect(page.getByTestId('feed-icon').first()).toHaveAttribute(
    'aria-hidden',
    'true',
  );
  await expect(
    page.getByTestId('feed-icon').first().locator('img'),
  ).toBeVisible();

  // A Feed Icon that fails to load falls back to the monogram rather than a
  // broken image. The icon response is cached aggressively (immutable), so
  // the browser's own HTTP cache is disabled first — otherwise a reload
  // would never re-request it, and the aborted route would never fire.
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Network.setCacheDisabled', { cacheDisabled: true });
  await page.route('**/api/feeds/*/icon*', (route) => route.abort());
  await page.reload();
  await expect(
    page.getByTestId('feed-icon').first().locator('img'),
  ).toHaveCount(0);
  await expect(page.getByTestId('feed-icon').first()).toContainText('T');
  await page.unroute('**/api/feeds/*/icon*');
  await page.reload();
  await expect(
    page.getByTestId('feed-icon').first().locator('img'),
  ).toBeVisible();
  await cdp.detach();

  // Newest first: Wheels, then Fire.
  const entries = page.getByTestId('entry');
  await expect(entries.nth(0)).toContainText('unread');
  await expect(entries.nth(1)).toContainText('unread');

  // Opening an Entry shows the Feed-supplied content, sanitised, and marks it
  // Read by default.
  await entries.nth(0).click();
  const drawer = page.getByTestId('entry-drawer');
  await expect(drawer).toBeVisible();
  await expect(page.getByTestId('entry-content')).toContainText(
    'Round, and it rolls.',
  );
  await expect(entries.nth(0)).not.toContainText('unread');

  // Next/previous from the keyboard, without leaving the drawer.
  await page.keyboard.press('j');
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );
  await expect(entries.nth(1)).not.toContainText('unread');

  await page.keyboard.press('k');
  await expect(page.getByTestId('entry-content')).toContainText(
    'Round, and it rolls.',
  );

  // Manual unread always overrides mark-on-open — including after navigating
  // away and back, which would otherwise re-trigger mark-on-open.
  await page.keyboard.press('m');
  await expect(entries.nth(0)).toContainText('unread');
  await page.keyboard.press('j');
  await page.keyboard.press('k');
  await expect(entries.nth(0)).toContainText('unread');

  // Close from the keyboard.
  await page.keyboard.press('Escape');
  await expect(drawer).toBeHidden();

  // The Unread filter scopes the list to what mark-on-open left unread.
  await page.getByTestId('filter-unread').click();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await expect(page.getByTestId('entry').first()).toContainText(
    'Wheels: a review',
  );
  await expect(page.getByTestId('feed-icon').first()).toBeVisible();

  await page.getByTestId('filter-all').click();
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // The `?` help dialog lists every binding from the one table.
  await page.keyboard.press('?');
  const help = page.getByTestId('help-dialog');
  await expect(help).toBeVisible();
  await expect(help).toContainText('Next Entry');
  await page.keyboard.press('Escape');
  await expect(help).toBeHidden();

  // Enter on a focused button is native activation, not the global Enter
  // binding: it must not silently open whatever the drawer last showed.
  await page.getByTestId('filter-unread').focus();
  await page.keyboard.press('Enter');
  await expect(drawer).toBeHidden();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await page.getByTestId('filter-all').click();

  // Star is optimistic and gives the Entry a dedicated view.
  await page.getByTestId('entry').first().click();
  await page.getByRole('button', { name: 'Star', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'Unstar', exact: true }),
  ).toBeVisible();
  await expect(page.getByTestId('entry').first()).toContainText('Starred');

  // A rejected Archive removes the Entry immediately, then restores it without
  // hijacking a different Entry the reader opened while the request was pending.
  await page.route('**/api/entries/*/state', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 250));
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'State rejected' }),
    });
  });
  await page.getByRole('button', { name: 'Archive', exact: true }).click();
  await expect(drawer).toBeHidden();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await page.getByTestId('entry').click();
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );
  await expect(page.getByTestId('notice')).toContainText('State rejected');
  await expect(page.getByTestId('entry')).toHaveCount(2);
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );
  await page.unroute('**/api/entries/*/state');
  await page.keyboard.press('Escape');

  // Archive disappears from Starred immediately, then appears only in Archive
  // with Read implied by the server-owned invariant.
  await page.getByTestId('filter-starred').click();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await expect(page.getByTestId('feed-icon').first()).toBeVisible();
  await page.getByTestId('entry').click();
  const archived = page.waitForResponse(
    (response) =>
      response.url().includes('/api/entries/') &&
      response.url().endsWith('/state') &&
      response.request().method() === 'PUT',
  );
  await page.getByRole('button', { name: 'Archive', exact: true }).click();
  await expect(drawer).toBeHidden();
  await expect(page.getByTestId('entry')).toHaveCount(0);
  await archived;
  await page.getByTestId('filter-archive').click();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await expect(page.getByTestId('entry')).toContainText('Archived');
  await expect(page.getByTestId('entry')).not.toContainText('unread');
  await expect(page.getByTestId('feed-icon').first()).toBeVisible();
  await page.getByTestId('entry').click();
  await expect(
    page.getByRole('button', { name: 'Unstar', exact: true }),
  ).toBeVisible();
  await page.keyboard.press('Escape');

  // Mark-all-read uses the current Unread filter and clears it optimistically.
  await page.getByTestId('filter-all').click();
  await page.getByTestId('entry').click();
  await page.getByRole('button', { name: 'Mark unread', exact: true }).click();
  await page.keyboard.press('Escape');
  await page.getByTestId('filter-unread').click();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await page
    .getByRole('button', { name: 'Mark all read', exact: true })
    .click();
  await expect(page.getByTestId('entry')).toHaveCount(0);

  // The browser suite shares one real database; restore the Archived Entry so
  // the subscription journey that follows still observes the publisher's two.
  await page.evaluate(async () => {
    const response = await fetch('/api/entries?archived=true');
    const body = (await response.json()) as { entries: { id: number }[] };
    await fetch(`/api/entries/${body.entries[0].id}/state`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ read: true, starred: false, archived: false }),
    });
  });
});
