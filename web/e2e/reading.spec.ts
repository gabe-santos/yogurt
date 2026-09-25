import { expect, test } from '@playwright/test';

import { password, publisherURL } from './env';
import { addFeed } from './actions';

test('the reader opens an Entry, reads it, and triages by keyboard', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  // Before any Feed exists, the Collection is genuinely empty — not merely
  // narrowed by a filter. The Collection's own name is still the page's
  // single h1, and there is no h2 since no Entry can be open.
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('h1')).toHaveAttribute('data-testid', 'collection');
  await expect(page.locator('h2')).toHaveCount(0);

  await addFeed(page, publisherURL);
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // The Collection's own name is the page's single h1; no Entry is open yet,
  // so there is no h2 either. See issue #37.
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('h1')).toHaveAttribute('data-testid', 'collection');
  await expect(page.locator('h2')).toHaveCount(0);

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

  // Right-click actions target the Entry under the pointer without opening it.
  const stateResponse = () =>
    page.waitForResponse(
      (response) =>
        response.url().includes('/api/entries/') &&
        response.url().endsWith('/state') &&
        response.request().method() === 'PUT',
    );
  await entries.nth(1).click({ button: 'right' });
  const entryMenu = page.getByTestId('entry-context-menu');
  await expect(
    entryMenu.getByRole('menuitem', { name: 'Mark read' }),
  ).toBeVisible();
  await expect(entryMenu.getByRole('menuitem', { name: 'Star' })).toBeVisible();
  await expect(
    entryMenu.getByRole('menuitem', { name: 'Archive' }),
  ).toBeVisible();
  let saved = stateResponse();
  await entryMenu.getByRole('menuitem', { name: 'Mark read' }).click();
  await saved;
  await expect(entries.nth(1)).not.toContainText('unread');

  await entries.nth(1).click({ button: 'right' });
  saved = stateResponse();
  await entryMenu.getByRole('menuitem', { name: 'Mark unread' }).click();
  await saved;
  await expect(entries.nth(1)).toContainText('unread');

  await entries.nth(1).click({ button: 'right' });
  saved = stateResponse();
  await entryMenu.getByRole('menuitem', { name: 'Star' }).click();
  await saved;
  await expect(entries.nth(1)).toContainText('Starred');

  await entries.nth(1).click({ button: 'right' });
  saved = stateResponse();
  await entryMenu.getByRole('menuitem', { name: 'Unstar' }).click();
  await saved;
  await expect(entries.nth(1)).not.toContainText('Starred');

  await page.route('**/api/entries/*/state', async (route) => {
    const delayed = Promise.withResolvers<void>();
    setTimeout(delayed.resolve, 150);
    await delayed.promise;
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'Context Archive rejected' }),
    });
  });
  await entries.nth(1).click({ button: 'right' });
  await entryMenu.getByRole('menuitem', { name: 'Archive' }).click();
  await expect(entries.nth(1)).toContainText('Archived');
  await expect(page.getByTestId('notice')).toContainText(
    'Context Archive rejected',
  );
  await expect(entries.nth(1)).not.toContainText('Archived');
  await expect(entries).toHaveCount(2);
  await page.unroute('**/api/entries/*/state');
  // Reload clears the session-only manual-unread override exercised above,
  // restoring the journey's ordinary mark-on-open starting state.
  await page.reload();
  await expect(entries).toHaveCount(2);
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

  // The open Entry's own title becomes the page's only h2, nested under the
  // Collection's own h1 — never a second h1. See issue #37.
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('h1')).toHaveAttribute('data-testid', 'collection');
  await expect(page.locator('h2')).toHaveCount(1);
  await expect(page.locator('h2')).toContainText('Wheels: a review');

  // Next/previous from the keyboard, without leaving the Reading Pane.
  await page.keyboard.press('j');
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );
  await expect(entries.nth(1)).not.toContainText('unread');
  // The reader's position moves with the keyboard, so the row j landed on
  // holds focus: that is what brings it into view in a longer list, and what
  // Tab continues from.
  await expect(entries.nth(1)).toBeFocused();

  await page.keyboard.press('k');
  await expect(page.getByTestId('entry-content')).toContainText(
    'Round, and it rolls.',
  );

  // Manual unread always overrides mark-on-open — including after navigating
  // away and back, which would otherwise re-trigger mark-on-open. Saving it
  // is held back so Unread Only below is chosen before the server has it: a
  // rebuilt list must still reflect what the reader already did.
  await page.route('**/api/entries/*/state', async (route) => {
    const held = Promise.withResolvers<void>();
    setTimeout(held.resolve, 500);
    await held.promise;
    await route.continue();
  });
  await page.keyboard.press('m');
  await expect(entries.nth(0)).toContainText('unread');
  await page.keyboard.press('j');
  await page.keyboard.press('k');
  await expect(entries.nth(0)).toContainText('unread');

  // Escape has nothing to close here: on a wide viewport the Reading Pane is a
  // column of the layout rather than something laid over the list.
  await page.keyboard.press('Escape');
  await expect(pane).toBeVisible();

  // Unread Only narrows the list to what mark-on-open left unread, from the
  // Entry List's own header rather than from a tab that owns the whole list.
  const unreadOnly = page.getByTestId('unread-only');
  await unreadOnly.click();
  await expect(unreadOnly).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByTestId('entry')).toHaveCount(1);
  // Narrowing rebuilds the list, which empties the Reading Pane.
  await expect(pane).toBeHidden();
  await expect(page.getByTestId('reading-pane-empty')).toBeVisible();
  await expect(page.getByTestId('entry').first()).toContainText(
    'Wheels: a review',
  );
  await expect(page.getByTestId('feed-icon').first()).toBeVisible();
  await page.unroute('**/api/entries/*/state');

  // u is the same act from the keyboard.
  await page.keyboard.press('u');
  await expect(unreadOnly).toHaveAttribute('aria-pressed', 'false');
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // The `?` help dialog lists every binding from the one table.
  await page.keyboard.press('?');
  const help = page.getByTestId('help-dialog');
  await expect(help).toBeVisible();
  await expect(help).toContainText('Next Entry');
  await expect(help).toContainText('Add a Feed');
  await page.keyboard.press('Escape');
  await expect(help).toBeHidden();

  // Enter is no longer a binding of its own — selecting an Entry is opening it
  // — so Enter on a focused control is plain native activation.
  await unreadOnly.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await page.keyboard.press('u');
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // Star is optimistic and gives the Entry a dedicated view.
  await page.getByTestId('entry').first().click();
  await page.getByRole('button', { name: 'Star', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'Unstar', exact: true }),
  ).toBeVisible();
  await expect(page.getByTestId('entry').first()).toContainText('Starred');

  // A rejected Archive is declared on the row it happened to and taken back
  // there. Nothing is removed, so nothing has to be put back in order, and the
  // Reading Pane is never handed an Entry the reader did not move to.
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
  await page.getByTestId('entry-archive').click();
  await expect(page.getByTestId('entry').first()).toContainText('Archived');
  await expect(page.getByTestId('notice')).toContainText('State rejected');
  await expect(page.getByTestId('entry').first()).not.toContainText('Archived');
  await expect(page.getByTestId('entry').first()).toContainText('Starred');
  await expect(page.getByTestId('entry')).toHaveCount(2);
  await expect(pane).toBeVisible();
  await page.unroute('**/api/entries/*/state');

  // Starred is a Collection in the Collection List, not a tab over the list.
  await page.getByTestId('collection-starred').click();
  await expect(page.getByTestId('collection')).toHaveText('Starred');
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await expect(page.getByTestId('feed-icon').first()).toBeVisible();
  await page.getByTestId('entry').click();
  const archived = page.waitForResponse(
    (response) =>
      response.url().includes('/api/entries/') &&
      response.url().endsWith('/state') &&
      response.request().method() === 'PUT',
  );
  await page.getByTestId('entry-archive').click();
  // Archiving the last Entry of a Collection does not empty it under the
  // reader: the row stays and says Archived.
  await expect(page.getByTestId('entry')).toContainText('Archived');
  await expect(pane).toBeVisible();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await archived;

  // The archive is the one Collection Unread Only is not offered in, because an
  // Archived Entry is always Read.
  await page.getByTestId('collection-archive').click();
  await expect(page.getByTestId('unread-only')).toBeHidden();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await expect(page.getByTestId('entry')).toContainText('Archived');
  await expect(page.getByTestId('entry')).not.toContainText('unread');
  await expect(page.getByTestId('feed-icon').first()).toBeVisible();
  await page.getByTestId('entry').click();
  await expect(
    page.getByRole('button', { name: 'Unstar', exact: true }),
  ).toBeVisible();

  // Mark-all-read declares the whole Collection Read without shortening it.
  await page.getByTestId('collection-all').click();
  await expect(page.getByTestId('unread-only')).toBeVisible();
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await page.getByTestId('entry').click();
  await page.getByRole('button', { name: 'Mark unread', exact: true }).click();
  await expect(page.getByTestId('entry')).toContainText('unread');
  await page
    .getByRole('button', { name: 'Mark all read', exact: true })
    .click();
  await expect(page.getByTestId('entry')).not.toContainText('unread');
  await expect(page.getByTestId('entry')).toHaveCount(1);
  await expect(page.getByTestId('notice')).toContainText(
    'Marked everything here Read.',
  );

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

// Runs against the two Entries reading.spec.ts's journey above leaves behind:
// both read, unstarred, unarchived. See issues #43 and #44.
test('archiving toggles from the Reading Pane, the context menu, and the keyboard, with rollback on rejection', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  const entries = page.getByTestId('entry');
  await expect(entries).toHaveCount(2);

  // e does nothing with no Entry selected.
  await page.keyboard.press('e');
  await expect(entries.first()).not.toContainText('Archived');
  await expect(entries.nth(1)).not.toContainText('Archived');

  const stateResponse = () =>
    page.waitForResponse(
      (response) =>
        response.url().includes('/api/entries/') &&
        response.url().endsWith('/state') &&
        response.request().method() === 'PUT',
    );

  // The Reading Pane control is a two-way toggle: Archive on an ordinary
  // Entry, Unarchive once it is Archived, without leaving the pane.
  await entries.first().click();
  const pane = page.getByTestId('reading-pane');
  await expect(pane).toBeVisible();
  await expect(page.getByTestId('entry-archive')).toHaveAccessibleName(
    'Archive',
  );
  let saved = stateResponse();
  await page.getByTestId('entry-archive').click();
  await saved;
  await expect(page.getByTestId('entry-archive')).toHaveAccessibleName(
    'Unarchive',
  );
  await expect(entries.first()).toContainText('Archived');
  // Mark read is disabled while Archived; unarchiving below re-enables it.
  await expect(
    page.getByRole('button', { name: 'Mark unread', exact: true }),
  ).toBeDisabled();

  // A rejected unarchive restores the Entry to Archived and surfaces the
  // error, leaving the row and the rest of the list untouched.
  await page.route('**/api/entries/*/state', async (route) => {
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'Unarchive rejected' }),
    });
  });
  await page.getByTestId('entry-archive').click();
  await expect(page.getByTestId('notice')).toContainText('Unarchive rejected');
  await expect(page.getByTestId('entry-archive')).toHaveAccessibleName(
    'Unarchive',
  );
  await expect(entries.first()).toContainText('Archived');
  await expect(entries).toHaveCount(2);
  await page.unroute('**/api/entries/*/state');

  // Unarchiving for real: the row holds still — same place, Archived marker
  // gone, Mark read available again — until the reader rebuilds the list.
  saved = stateResponse();
  await page.getByTestId('entry-archive').click();
  await saved;
  await expect(page.getByTestId('entry-archive')).toHaveAccessibleName(
    'Archive',
  );
  await expect(entries.first()).not.toContainText('Archived');
  await expect(entries.first()).not.toContainText('unread');
  await expect(entries).toHaveCount(2);
  await expect(
    page.getByRole('button', { name: 'Mark unread', exact: true }),
  ).toBeEnabled();
  await expect(pane).toBeVisible();

  // The Entry List context menu carries the same toggle.
  await entries.nth(1).click({ button: 'right' });
  const entryMenu = page.getByTestId('entry-context-menu');
  await expect(
    entryMenu.getByRole('menuitem', { name: 'Archive', exact: true }),
  ).toBeVisible();
  saved = stateResponse();
  await entryMenu
    .getByRole('menuitem', { name: 'Archive', exact: true })
    .click();
  await saved;
  await expect(entries.nth(1)).toContainText('Archived');
  await entries.nth(1).click({ button: 'right' });
  await expect(
    entryMenu.getByRole('menuitem', { name: 'Unarchive', exact: true }),
  ).toBeVisible();
  saved = stateResponse();
  await entryMenu
    .getByRole('menuitem', { name: 'Unarchive', exact: true })
    .click();
  await saved;
  await expect(entries.nth(1)).not.toContainText('Archived');

  // The `?` help dialog documents the binding, straight from the one table.
  await page.keyboard.press('?');
  const help = page.getByTestId('help-dialog');
  await expect(help).toBeVisible();
  await expect(help).toContainText('Archive / unarchive');
  await page.keyboard.press('Escape');
  await expect(help).toBeHidden();

  // e toggles Archived on the selected Entry from the keyboard, exactly like
  // the pointer path — same Read rules, same rollback, same held-still list.
  await entries.first().click();
  await expect(pane).toBeVisible();
  saved = stateResponse();
  await page.keyboard.press('e');
  await saved;
  await expect(entries.first()).toContainText('Archived');
  await expect(page.getByTestId('entry-archive')).toHaveAccessibleName(
    'Unarchive',
  );
  // Like its buttons, an Entry ignores keys until its last change is saved,
  // and the response arriving is not yet the app having taken it in.
  await expect(page.getByTestId('entry-archive')).toBeEnabled();
  saved = stateResponse();
  await page.keyboard.press('e');
  await saved;
  await expect(entries.first()).not.toContainText('Archived');
  await expect(entries.first()).not.toContainText('unread');
  await expect(page.getByTestId('entry-archive')).toHaveAccessibleName(
    'Archive',
  );

  // Unarchiving inside the archive Collection itself holds the row still —
  // same place, Archived marker gone — and only actually leaves once the
  // reader rebuilds the list, never on its own.
  saved = stateResponse();
  await page.getByTestId('entry-archive').click();
  await saved;
  await page.getByTestId('collection-archive').click();
  await expect(entries).toHaveCount(1);
  await expect(entries).toContainText('Archived');
  await entries.first().click();
  await expect(pane).toBeVisible();
  saved = stateResponse();
  await page.getByTestId('entry-archive').click();
  await saved;
  await expect(entries).toHaveCount(1);
  await expect(entries).not.toContainText('Archived');
  await expect(entries).not.toContainText('unread');
  await expect(pane).toBeVisible();
  await page.getByTestId('collection-archive').click();
  await expect(entries).toHaveCount(0);
  await page.getByTestId('collection-all').click();
  await expect(entries).toHaveCount(2);

  // Inert while the reader is typing in a dialog's field.
  await entries.first().click();
  await expect(pane).toBeVisible();
  await expect(page.getByTestId('entry-archive')).toHaveAccessibleName(
    'Archive',
  );
  const feed = page.getByTestId('feed');
  const dialog = page.getByTestId('edit-feed-dialog');
  await feed.click({ button: 'right' });
  await page
    .getByTestId('feed-context-menu')
    .getByRole('menuitem', { name: 'Edit' })
    .click();
  const name = dialog.getByLabel('Name');
  await name.clear();
  await page.keyboard.type('e');
  await expect(name).toHaveValue('e');
  // Escape closes the dialog without saving the draft.
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(feed).toHaveText('The Daily Cave');
  await expect(page.getByTestId('entry-archive')).toHaveAccessibleName(
    'Archive',
  );
});
