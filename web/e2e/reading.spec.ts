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

  // Publish-date order defaults to newest first and can be reversed without
  // changing the current Feed scope.
  const entries = page.getByTestId('entry');
  const entryOrder = page.getByTestId('entry-order');
  await expect(entryOrder).toContainText('Newest first');
  await expect(entries.nth(0)).toContainText('Wheels: a review');
  await expect(entries.nth(1)).toContainText('Fire, and how to keep it');
  await entryOrder.click();
  await page.getByRole('option', { name: 'Oldest first' }).click();
  await expect(entryOrder).toContainText('Oldest first');
  await expect(entries.nth(0)).toContainText('Fire, and how to keep it');
  await expect(entries.nth(1)).toContainText('Wheels: a review');
  await entryOrder.click();
  await page.getByRole('option', { name: 'Newest first' }).click();
  await expect(entryOrder).toContainText('Newest first');

  await expect(entries.nth(0)).toContainText('Wheels: a review');
  await expect(entries.nth(0)).toContainText('unread');
  await expect(entries.nth(1)).toContainText('unread');

  // Opening an Entry shows the Feed-supplied content, sanitised, and marks it
  // Read by default.
  await entries.nth(0).click();
  const pane = page.getByTestId('reading-pane');
  await expect(pane).toBeVisible();
  await expect(page.getByTestId('entry-content')).toContainText(
    'Round, and it rolls.',
  );
  await expect(entries.nth(0)).not.toContainText('unread');

  // Next/previous from the keyboard, without leaving the Reading Pane.
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

  // Escape has nothing to close here: on a wide viewport the Reading Pane is a
  // column of the layout rather than something laid over the list.
  await page.keyboard.press('Escape');
  await expect(pane).toBeVisible();

  // The Unread filter scopes the list to what mark-on-open left unread.
  await page.getByTestId('filter-unread').click();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  // Changing the filter reloads the list, which empties the Reading Pane.
  await expect(pane).toBeHidden();
  await expect(page.getByTestId('reading-pane-empty')).toBeVisible();
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

  // Enter is no longer a binding of its own — selecting an Entry is opening it
  // — so Enter on a focused control is plain native activation.
  await page.getByTestId('filter-unread').focus();
  await page.keyboard.press('Enter');
  await expect(pane).toBeHidden();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await page.getByTestId('filter-all').click();

  // Star is optimistic and gives the Entry a dedicated view.
  await page.getByTestId('entry').first().click();
  await page.getByRole('button', { name: 'Star', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'Unstar', exact: true }),
  ).toBeVisible();
  await expect(page.getByTestId('entry').first()).toContainText('Starred');

  // A rejected Archive removes the Entry immediately and moves the reader on to
  // the next one, then restores it without hijacking the Entry the Reading Pane
  // has moved to.
  await page.route('**/api/entries/*/state', async (route) => {
    const delayed = Promise.withResolvers<void>();
    setTimeout(delayed.resolve, 250);
    await delayed.promise;
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'State rejected' }),
    });
  });
  await page.getByRole('button', { name: 'Archive', exact: true }).click();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );
  await expect(page.getByTestId('notice')).toContainText('State rejected');
  await expect(page.getByTestId('entry')).toHaveCount(2);
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );
  await page.unroute('**/api/entries/*/state');

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
  // Archiving the last Entry of a view leaves nothing to move on to.
  await expect(pane).toBeHidden();
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

  // Mark-all-read uses the current Unread filter and clears it optimistically.
  await page.getByTestId('filter-all').click();
  await page.getByTestId('entry').click();
  await page.getByRole('button', { name: 'Mark unread', exact: true }).click();
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
