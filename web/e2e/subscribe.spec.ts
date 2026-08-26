import { expect, test } from '@playwright/test';

import { password, publisherURL } from './env';

test('the reader subscribes to a site and reads what it published', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByLabel('Feed or site address')).toBeVisible();

  // The site's own address, not its Feed: the app finds the Feed itself.
  await page.getByLabel('Feed or site address').fill(publisherURL);
  await page.getByRole('button', { name: 'Subscribe' }).click();

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

  // Subscribing again is refused, with a reason.
  await page
    .getByLabel('Feed or site address')
    .fill(`${publisherURL}/feed.xml`);
  await page.getByRole('button', { name: 'Subscribe' }).click();
  await expect(page.getByRole('alert')).toContainText('already subscribed');
  await expect(page.getByTestId('feed')).toHaveCount(1);

  // A refresh on demand changes nothing when the publisher has published
  // nothing, and says so rather than leaving the reader guessing.
  await page.getByRole('button', { name: 'Refresh all' }).click();
  await expect(page.getByTestId('notice')).toHaveText(
    'Every Feed is up to date.',
  );
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // The Feed Icon slot (a monogram, until a real icon exists) survives
  // scoping to the Feed's Group and appears in search results too.
  await page.getByTestId('group').first().click();
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

test('the reader manages a Feed and a Group from their right-click menus', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();

  await page.getByLabel('Feed or site address').fill(publisherURL);
  await page.getByRole('button', { name: 'Subscribe' }).click();

  const feed = page.getByTestId('feed');
  const feedMenu = page.getByTestId('feed-context-menu');
  const groupMenu = page.getByTestId('group-context-menu');
  const feedSaved = () =>
    page.waitForResponse(
      (response) =>
        /\/api\/feeds\/\d+$/.test(response.url()) &&
        response.request().method() === 'PUT',
    );
  await expect(feed).toHaveText('The Daily Cave');

  // Every act the Feed List offers is on the Feed's own menu.
  await feed.click({ button: 'right' });
  await expect(
    feedMenu.getByRole('menuitem', { name: 'Rename' }),
  ).toBeVisible();
  await expect(
    feedMenu.getByRole('menuitem', { name: 'Move to Group' }),
  ).toBeVisible();
  await expect(
    feedMenu.getByRole('menuitem', { name: 'Delete Feed' }),
  ).toBeVisible();

  // Renaming opens the field with the current name selected, so the new one is
  // typed straight over it without the reader reaching for the mouse again.
  await feedMenu.getByRole('menuitem', { name: 'Rename' }).click();
  await expect(page.getByLabel('Rename the Feed The Daily Cave')).toBeFocused();
  let saved = feedSaved();
  await page.keyboard.type('Cave Chronicle');
  await page.keyboard.press('Enter');
  await saved;
  await expect(feed).toHaveText('Cave Chronicle');

  // A second Group to move the Feed into.
  await page.getByTestId('manage-feeds').click();
  await page.getByLabel('New Group').fill('Deep Reads');
  await page.getByRole('button', { name: 'Add' }).click();
  await expect(page.getByTestId('group')).toHaveCount(2);
  await page.getByTestId('manage-feeds').click();

  const deepReads = page.getByTestId('group').filter({ hasText: 'Deep Reads' });
  await feed.click({ button: 'right' });
  await feedMenu.getByRole('menuitem', { name: 'Move to Group' }).hover();
  saved = feedSaved();
  await page.getByRole('menuitemradio', { name: 'Deep Reads' }).click();
  await saved;
  // Moved for real: the Group it landed in now scopes to the Feed's Entries.
  await deepReads.click();
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // A Group carries its own menu, holding the acts that are its own.
  await deepReads.click({ button: 'right' });
  await groupMenu.getByRole('menuitem', { name: 'Rename' }).click();
  await expect(page.getByLabel('Rename the Group Deep Reads')).toBeFocused();
  await page.keyboard.type('Long Reads');
  await page.keyboard.press('Enter');
  const longReads = page.getByTestId('group').filter({ hasText: 'Long Reads' });
  await expect(longReads).toHaveCount(1);

  // The default Group is where an unsorted Feed lives, so its menu offers no
  // way to delete it.
  await page.getByTestId('group').filter({ hasText: 'Unsorted' }).click({
    button: 'right',
  });
  await expect(
    groupMenu.getByRole('menuitem', { name: 'Rename' }),
  ).toBeVisible();
  await expect(
    groupMenu.getByRole('menuitem', { name: 'Delete Group' }),
  ).toHaveCount(0);
  await page.keyboard.press('Escape');

  // Deleting a Group hands its Feeds back to the default one.
  await longReads.click({ button: 'right' });
  await groupMenu.getByRole('menuitem', { name: 'Delete Group' }).click();
  await expect(page.getByTestId('confirm-dialog')).toContainText('Long Reads');
  await page.getByTestId('confirm-dialog-confirm').click();
  await expect(page.getByTestId('group')).toHaveCount(1);
  await feed.click({ button: 'right' });
  await feedMenu.getByRole('menuitem', { name: 'Move to Group' }).hover();
  await expect(
    page.getByRole('menuitemradio', { name: 'Unsorted' }),
  ).toHaveAttribute('aria-checked', 'true');
  await page.keyboard.press('Escape');

  // Deleting the Feed is asked first, and cancelling keeps it.
  await feed.click({ button: 'right' });
  await feedMenu.getByRole('menuitem', { name: 'Delete Feed' }).click();
  await expect(page.getByTestId('confirm-dialog')).toContainText(
    'Every Entry it carried is deleted with it',
  );
  await expect(page.getByTestId('confirm-dialog')).toContainText(
    'Re-subscribing starts the Feed from scratch.',
  );
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(feed).toHaveCount(1);

  // Left as it was found, name included.
  await feed.click({ button: 'right' });
  await feedMenu.getByRole('menuitem', { name: 'Rename' }).click();
  saved = feedSaved();
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

  await page.getByLabel('Feed or site address').fill(publisherURL);
  await page.getByRole('button', { name: 'Subscribe' }).click();

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
    feedMenu.getByRole('menuitem', { name: 'Move to Group' }),
  ).toBeVisible();
  await expect(
    feedMenu.getByRole('menuitem', { name: 'Delete Feed' }),
  ).toBeVisible();

  // Subscribing fetches the Feed once, so its health line already states
  // that rather than "Not checked yet".
  await expect(feedMenu).toContainText(/^Checked /);
  await page.keyboard.press('Escape');
});
