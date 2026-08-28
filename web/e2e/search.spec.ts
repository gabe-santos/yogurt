import { expect, test } from '@playwright/test';

import { password, publisherURL } from './env';
import { addFeed } from './actions';

// removeFeedNamed deletes the named Feed, if any. The suite shares one
// database, so this spec's isolated Feed must not leak into the journeys
// that run after it — including when an assertion above fails mid-test.
async function removeFeedNamed(page: import('@playwright/test').Page, title: string) {
  return page.evaluate(async (title) => {
    const response = await fetch('/api/feeds');
    const body = (await response.json()) as {
      feeds: { id: number; title: string }[];
    };
    const named = body.feeds.find((f) => f.title === title);
    if (!named) return false;
    const result = await fetch(`/api/feeds/${named.id}`, { method: 'DELETE' });
    return result.ok;
  }, title);
}

// See issue #38: the dialog's live region must exist before content
// changes, not appear with them, so a screen reader actually hears the
// transition. Subscribes to the second, isolated publisher Feed — never the
// one reading.spec.ts already subscribed — so this spec runs independently
// of suite ordering, and deletes it afterwards (even on failure) since the
// suite shares one database.
test('search announces its state transitions to assistive tech', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  await addFeed(page, `${publisherURL}/second.xml`, 'Only Post Digest');
  try {
    await expect(
      page.getByTestId('feed').filter({ hasText: 'Only Post Digest' }),
    ).toBeVisible();
    await expect(page.getByTestId('entry')).toHaveCount(1);

    const status = page.getByTestId('search-status');
    const errorStatus = page.getByTestId('search-error-status');

    await page.getByTestId('open-search').click();

    // Both live regions exist before any query is typed — the announcing
    // region must be present ahead of the change, not appear alongside it.
    await expect(status).toHaveAttribute('aria-live', 'polite');
    await expect(errorStatus).toHaveAttribute('aria-live', 'assertive');
    await expect(status).toHaveText('');
    await expect(errorStatus).toHaveText('');

    // Delay the response so the transient "Searching…" state is observable.
    await page.route('**/api/search*', async (route) => {
      const { promise, resolve } = Promise.withResolvers<void>();
      setTimeout(resolve, 300);
      await promise;
      await route.continue();
    });
    await page.getByTestId('search-input').fill('Only post');
    await expect(status).toHaveText('Searching…');
    await expect(page.getByTestId('search-result')).toHaveCount(2);
    await expect(status).toHaveText('2 results found.');
    await page.unroute('**/api/search*');

    // A query naming nothing announces the no-match line by name, politely.
    await page.getByTestId('search-input').fill('Only post but matches nothing');
    await expect(status).toHaveText(
      'No matches for "Only post but matches nothing".',
    );
    await expect(
      page.getByTestId('search-dialog').getByRole('paragraph'),
    ).toHaveText('No matches for "Only post but matches nothing".');

    // A failed request is announced assertively, and clears the polite region.
    await page.route('**/api/search*', (route) => route.abort());
    await page.getByTestId('search-input').fill('Only post');
    await expect(errorStatus).toHaveText(
      'Could not reach the server. Check your connection and try again.',
    );
    await expect(status).toHaveText('');
    await page.unroute('**/api/search*');

    // Existing keyboard behaviour survives: arrow keys move the active
    // option, and Enter opens it. Two results — the Feed and its Entry — so
    // ArrowDown genuinely moves aria-activedescendant rather than wrapping a
    // single-item list back to itself.
    const input = page.getByTestId('search-input');
    await input.fill('Only post');
    await expect(page.getByTestId('search-result')).toHaveCount(2);
    const first = await input.getAttribute('aria-activedescendant');
    await page.keyboard.press('ArrowDown');
    await expect(input).not.toHaveAttribute('aria-activedescendant', first ?? '');
    await page.keyboard.press('ArrowUp');
    await expect(input).toHaveAttribute('aria-activedescendant', first ?? '');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('search-dialog')).toHaveCount(0);
    await expect(page.getByRole('heading', { level: 2 })).toContainText(
      'Only post',
    );
  } finally {
    // The suite shares one database; remove the Feed so the journeys after
    // this one keep seeing only the Feed reading.spec.ts subscribed to,
    // even if an assertion above failed.
    await removeFeedNamed(page, 'Only Post Digest');
  }
});
