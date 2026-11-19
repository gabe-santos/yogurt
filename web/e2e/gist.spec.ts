import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';

import { password, publisherURL } from './env';

// clearAIConfig clears the Gist configuration directly, so the suite's
// shared database starts (and ends) unconfigured even if an assertion above
// fails mid-test.
async function clearAIConfig(page: Page) {
  return page.evaluate(() => fetch('/api/ai', { method: 'DELETE' }));
}

// subscribeViaAPI adds a Feed without going through the Add Feed dialog —
// this spec's subject is the Gist block, not the subscribe flow, which
// reading.spec.ts and subscribe.spec.ts already cover.
async function subscribeViaAPI(page: Page, url: string) {
  return page.evaluate(async (feedURL) => {
    const response = await fetch('/api/feeds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: feedURL }),
    });
    const body = (await response.json()) as { feed: { id: number } };
    return body.feed;
  }, url);
}

async function deleteFeed(page: Page, id: number) {
  return page.evaluate(
    (feedID) => fetch(`/api/feeds/${feedID}`, { method: 'DELETE' }),
    id,
  );
}

// This spec runs before reading.spec.ts, which asserts no Feed exists yet —
// so every Feed it subscribes to here is removed again in `finally`, leaving
// the Instance exactly as unconfigured and empty as it found it.
test('the reader requests a Gist with `g` and the button over every view, keeps it, regenerates it, and sees a failure as inert text shown with Retry', async ({
  page,
}) => {
  // Counts POSTs to any Entry's Gist endpoint, so the off-state check below
  // can prove `g` sent nothing — not just that the block stayed hidden.
  let gistPosts = 0;
  page.on('request', (request) => {
    if (request.method() === 'POST' && /\/api\/entries\/\d+\/gist$/.test(request.url())) {
      gistPosts++;
    }
  });

  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('button', { name: 'Add Feed' })).toBeVisible();

  // Two Feeds, three fresh Entries with no Gist yet — one per view this
  // journey requests one in.
  const feedA = await subscribeViaAPI(page, `${publisherURL}/feed.xml`);
  const feedB = await subscribeViaAPI(page, `${publisherURL}/second.xml`);

  try {
    await page.reload();
    await expect(page.getByTestId('entry')).toHaveCount(3);
    const entries = page.getByTestId('entry');
    const fireEntry = entries.filter({ hasText: 'Fire' });
    const wheelsEntry = entries.filter({ hasText: 'Wheels' });
    const secondEntry = entries.filter({ hasText: 'Only post' });

    await fireEntry.click();

    // Before Gists are configured, nothing about them appears, and `g` does
    // nothing — not even a request. The `?` help dialog hides the row for
    // it too.
    await expect(page.getByTestId('gist-block')).toBeHidden();
    await page.keyboard.press('g');
    await expect(page.getByTestId('gist-block')).toBeHidden();
    expect(gistPosts).toBe(0);
    await page.keyboard.press('?');
    const helpDialog = page.getByTestId('help-dialog');
    await expect(helpDialog).toBeVisible();
    await expect(helpDialog.getByText('Request the Gist')).toBeHidden();
    await page.keyboard.press('Escape');
    await expect(helpDialog).toBeHidden();

    // Configure Gists in the Yogurt Menu (GABE-202), at first pointing at an
    // endpoint that cannot answer.
    await page.getByTestId('yogurt-menu').click();
    await page.getByRole('menuitem', { name: 'Configure Gists' }).click();
    const configDialog = page.getByTestId('gist-config-dialog');
    await expect(configDialog).toBeVisible();
    await configDialog
      .getByLabel('Endpoint URL')
      .fill(`${publisherURL}/v1/wrong-path`);
    await configDialog.getByLabel('Model').fill('test-model');
    const saveButton = configDialog.getByRole('button', { name: 'Save' });
    await saveButton.click();
    // The Save button disables itself for the length of its own request;
    // waiting for it to re-enable proves that write landed, rather than
    // racing a response.
    await expect(saveButton).toBeEnabled();
    await page.getByRole('button', { name: 'Close' }).first().click();

    // Feed View, `g`: reopening Fire now offers a Gist; requesting one
    // against the broken endpoint fails with a Retry rather than hanging or
    // getting stuck loading.
    await fireEntry.click();
    await expect(page.getByTestId('view-feed')).toHaveAttribute('data-state', 'active');
    await expect(page.getByTestId('gist-block')).toBeVisible();
    await page.keyboard.press('g');
    await expect(page.getByTestId('gist-error')).toBeVisible();
    const retry = page.getByTestId('gist-retry');
    await expect(retry).toBeVisible();

    // Fixing the configuration and pressing Retry (the button, still Feed
    // View) succeeds.
    await page.getByTestId('yogurt-menu').click();
    await page.getByRole('menuitem', { name: 'Configure Gists' }).click();
    await configDialog
      .getByLabel('Endpoint URL')
      .fill(`${publisherURL}/v1/chat/completions`);
    await saveButton.click();
    await expect(saveButton).toBeEnabled();
    await page.getByRole('button', { name: 'Close' }).first().click();
    await retry.click();
    const bullets = page.getByTestId('gist-bullets');
    await expect(bullets).toBeVisible();

    // The model's reply is shown as inert text: a script tag never becomes
    // an element, and a Markdown-style link never becomes a clickable one.
    await expect(bullets).toContainText('dangerous link');
    await expect(bullets.locator('script')).toHaveCount(0);
    await expect(bullets.locator('a')).toHaveCount(0);

    // `g` collapses the shown Gist, and again expands it — still Feed View.
    await page.keyboard.press('g');
    await expect(bullets).toBeHidden();
    await page.keyboard.press('g');
    await expect(bullets).toBeVisible();

    // Regenerate (the button) asks again, from Feed View, and still
    // succeeds.
    await page.getByTestId('gist-regenerate').click();
    await expect(page.getByTestId('gist-bullets')).toBeVisible();

    // Now that Gists are on, the `?` help dialog shows the row for `g`.
    await page.keyboard.press('?');
    await expect(helpDialog).toBeVisible();
    await expect(helpDialog.getByText('Request the Gist')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(helpDialog).toBeHidden();

    // Reader View, the button: a different, still-idle Entry gets its first
    // Gist with Get Gist rather than `g`.
    await page.getByTestId('view-reader').click();
    await wheelsEntry.click();
    await expect(page.getByTestId('gist-block')).toBeVisible();
    await expect(page.getByTestId('gist-request')).toBeVisible();
    await page.getByTestId('gist-request').click();
    await expect(page.getByTestId('gist-bullets')).toBeVisible();

    // Original View, `g`: a third, still-idle Entry (a different Feed, so
    // Original View has something of its own to embed) gets its first Gist
    // with the keyboard instead of the button.
    await page.getByTestId('view-original').click();
    await secondEntry.click();
    await expect(page.getByTestId('gist-block')).toBeVisible();
    await page.keyboard.press('g');
    await expect(page.getByTestId('gist-bullets')).toBeVisible();

    // Fire's Gist is kept: leaving it and coming back, and reloading
    // outright, both show it again without a fresh request.
    await page.getByTestId('view-feed').click();
    await fireEntry.click();
    await expect(bullets).toBeVisible();
    await wheelsEntry.click();
    await fireEntry.click();
    await expect(bullets).toBeVisible();
    await page.reload();
    await expect(page.getByTestId('gist-bullets')).toBeVisible();

    // Clearing the configuration hides the block entirely, even though every
    // Gist above is still stored.
    await page.getByTestId('yogurt-menu').click();
    await page.getByRole('menuitem', { name: 'Configure Gists' }).click();
    const turnOffButton = configDialog.getByRole('button', { name: 'Turn off Gists' });
    await turnOffButton.click();
    await expect(turnOffButton).toBeEnabled();
    await page.getByRole('button', { name: 'Close' }).first().click();
    await expect(page.getByTestId('gist-block')).toBeHidden();
  } finally {
    await clearAIConfig(page);
    await deleteFeed(page, feedA.id);
    await deleteFeed(page, feedB.id);
  }
});
