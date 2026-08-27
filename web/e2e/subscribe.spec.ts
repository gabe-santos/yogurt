import { expect, test } from '@playwright/test';

import { password, publisherURL } from './env';
import { addFeed } from './actions';

test('the reader subscribes to a site and reads what it published', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  // The suite shares one database, and reading.spec.ts already subscribed to
  // this publisher: subscribing again — by the site's own address, not its
  // Feed — is refused, with the reason beside the field that caused it, and
  // the address stays put so it can be corrected. The dialog stays open.
  const dialog = page.getByTestId('add-feed-dialog');
  await addFeed(page, publisherURL);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('alert')).toContainText(
    'already have this Feed',
  );
  await expect(dialog.getByLabel('Feed or site address')).toHaveValue(
    publisherURL,
  );
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  await expect(page.getByTestId('feed')).toHaveText('The Daily Cave');
  // The sidebar's Feed Icon was discovered at subscribe time, not just
  // rendered as the fallback monogram.
  await expect(
    page.getByTestId('feed').getByTestId('feed-icon').locator('img'),
  ).toBeVisible();
  await expect(page.getByTestId('entry')).toHaveCount(2);
  // Newest first.
  await expect(page.getByTestId('entry').first()).toContainText(
    'Wheels: a review',
  );

  // Subscribing to the Feed's own address, rather than the site, is refused
  // the same way.
  await addFeed(page, `${publisherURL}/feed.xml`);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('alert')).toContainText(
    'already have this Feed',
  );
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(page.getByTestId('feed')).toHaveCount(1);

  // A refresh on demand changes nothing when the publisher has published
  // nothing, and says so rather than leaving the reader guessing.
  await page.getByRole('button', { name: 'Refresh all' }).click();
  await expect(page.getByTestId('notice')).toHaveText(
    'Every Feed is up to date.',
  );
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // The Feed Icon slot (a monogram, until a real icon exists) survives
  // scoping back to "All Feeds" and appears in search results too.
  await page.getByRole('button', { name: 'All Feeds' }).click();
  await expect(page.getByTestId('entry')).toHaveCount(2);
  await expect(page.getByTestId('feed-icon').first()).toBeVisible();

  await page.getByTestId('open-search').click();
  await page.getByTestId('search-input').fill('Wheels');
  await expect(page.getByTestId('search-result')).toHaveCount(1);
  await expect(
    page.getByTestId('search-result').getByTestId('feed-icon'),
  ).toBeVisible();
  await page.keyboard.press('Escape');

  // The list survives a reload, because the Entries are on the server.
  await page.reload();
  await expect(page.getByTestId('entry')).toHaveCount(2);
});

test('the reader names a Feed while subscribing', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  // `a` opens the dialog exactly like the "+" action does.
  await page.keyboard.press('a');
  const dialog = page.getByTestId('add-feed-dialog');
  await expect(dialog).toBeVisible();
  await dialog
    .getByLabel('Feed or site address')
    .fill(`${publisherURL}/second.xml`);
  await dialog.getByLabel('Name').fill('My Second Feed');
  await dialog.getByRole('button', { name: 'Add Feed' }).click();
  await expect(dialog).toBeHidden();

  const feed = page.getByTestId('feed').filter({ hasText: 'My Second Feed' });
  await expect(feed).toBeVisible();
  await expect(page.getByTestId('collection')).toHaveText('My Second Feed');
  await expect(page.getByTestId('notice')).toHaveText(
    'Added Feed My Second Feed.',
  );

  // The suite shares one database; remove the Feed so the journeys after
  // this one keep seeing only the Feed reading.spec.ts subscribed to.
  const deleted = await page.evaluate(async () => {
    const response = await fetch('/api/feeds');
    const body = (await response.json()) as {
      feeds: { id: number; title: string }[];
    };
    const named = body.feeds.find((f) => f.title === 'My Second Feed');
    if (!named) return false;
    const result = await fetch(`/api/feeds/${named.id}`, {
      method: 'DELETE',
    });
    return result.ok;
  });
  expect(deleted).toBe(true);
});

test('the reader manages a Feed from its right-click menu', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();

  const feed = page.getByTestId('feed');
  const feedMenu = page.getByTestId('feed-context-menu');
  const feedPutSaved = (response) =>
    /\/api\/feeds\/\d+$/.test(response.url()) &&
    response.request().method() === 'PUT';
  await expect(feed).toHaveText('The Daily Cave');

  // Every act the Collection List offers is on the Feed's own menu.
  await feed.click({ button: 'right' });
  await expect(
    feedMenu.getByRole('menuitem', { name: 'Rename' }),
  ).toBeVisible();
  await expect(
    feedMenu.getByRole('menuitem', { name: 'Delete Feed' }),
  ).toBeVisible();

  // Renaming opens the field with the current name selected, so the new one is
  // typed straight over it without the reader reaching for the mouse again.
  await feedMenu.getByRole('menuitem', { name: 'Rename' }).click();
  await expect(page.getByLabel('Rename the Feed The Daily Cave')).toBeFocused();
  let saved = page.waitForResponse(feedPutSaved);
  await page.keyboard.type('Cave Chronicle');
  await page.keyboard.press('Enter');
  await saved;
  await expect(feed).toHaveText('Cave Chronicle');

  // Deleting the Feed is asked first, and cancelling keeps it.
  await feed.click({ button: 'right' });
  await feedMenu.getByRole('menuitem', { name: 'Delete Feed' }).click();
  await expect(page.getByTestId('confirm-dialog')).toContainText(
    'Every Entry it carried is deleted with it',
  );
  await expect(page.getByTestId('confirm-dialog')).toContainText(
    'Adding the Feed again starts it from scratch.',
  );
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(feed).toHaveCount(1);

  // Left as it was found, name included.
  await feed.click({ button: 'right' });
  await feedMenu.getByRole('menuitem', { name: 'Rename' }).click();
  saved = page.waitForResponse(feedPutSaved);
  await page.keyboard.type('The Daily Cave');
  await page.keyboard.press('Enter');
  await saved;
  await expect(feed).toHaveText('The Daily Cave');
});

test('the "…" menu button opens the same menu as right-click, reporting Feed health', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();

  const feed = page.getByTestId('feed');
  const feedMenu = page.getByTestId('feed-menu');
  await expect(feed).toHaveText('The Daily Cave');

  // The action button is named after the Feed, not left generic, and opens
  // the identical menu right-click already opens.
  await page.getByRole('button', { name: 'The Daily Cave menu' }).click();
  await expect(feedMenu).toBeVisible();
  await expect(
    feedMenu.getByRole('menuitem', { name: 'Rename' }),
  ).toBeVisible();
  await expect(
    feedMenu.getByRole('menuitem', { name: 'Delete Feed' }),
  ).toBeVisible();

  // Subscribing fetches the Feed once, so its health line already states
  // that rather than "Not checked yet".
  await expect(feedMenu).toContainText(/^Checked /);
  await page.keyboard.press('Escape');
});

test('a failing Feed offers its full error from either menu, by keyboard', async ({
  page,
}) => {
  const longError =
    'Error: fetch failed\n'.repeat(4) +
    'caused by: context deadline exceeded while reading the response body';

  await page.route('**/api/feeds', async (route) => {
    if (route.request().method() !== 'GET') return route.continue();
    const response = await route.fetch();
    const body = await response.json();
    body.feeds = body.feeds.map((feed: { last_error: string }) => ({
      ...feed,
      last_error: longError,
    }));
    await route.fulfill({ response, json: body });
  });

  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();

  const feed = page.getByTestId('feed');
  const errorDialog = page.getByTestId('feed-error-dialog');
  await expect(feed).toHaveText('The Daily Cave');

  // The right-click menu's clamped preview stays two lines — the full text
  // is reachable, not inlined into the menu.
  const contextMenu = page.getByTestId('feed-context-menu');
  await feed.click({ button: 'right' });
  await expect(contextMenu).toBeVisible();
  await expect(
    contextMenu.getByRole('menuitem', { name: 'View full error' }),
  ).toBeVisible();

  // Reached by keyboard alone, no mouse hover involved — Enter activates
  // the item once it holds focus, the same way Rename and Delete Feed do.
  await contextMenu.getByRole('menuitem', { name: 'View full error' }).press('Enter');
  await expect(errorDialog).toBeVisible();
  await expect(errorDialog).toContainText(
    'context deadline exceeded while reading the response body',
  );
  await page.keyboard.press('Escape');
  await expect(errorDialog).toBeHidden();

  // The "…" button menu offers the identical control — one shared body.
  const dropdownMenu = page.getByTestId('feed-menu');
  await page.getByRole('button', { name: 'The Daily Cave menu' }).click();
  await expect(dropdownMenu).toBeVisible();
  await dropdownMenu.getByRole('menuitem', { name: 'View full error' }).click();
  await expect(errorDialog).toBeVisible();
  await expect(errorDialog).toContainText(
    'context deadline exceeded while reading the response body',
  );
  await page.keyboard.press('Escape');
  await expect(errorDialog).toBeHidden();

  // A Feed with no failure offers no such control.
  await page.unroute('**/api/feeds');
  await page.reload();
  await expect(feed).toHaveText('The Daily Cave');
  await feed.click({ button: 'right' });
  await expect(
    contextMenu.getByRole('menuitem', { name: 'View full error' }),
  ).toHaveCount(0);
});
