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
