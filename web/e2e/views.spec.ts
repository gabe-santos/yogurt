import { expect, test } from '@playwright/test';

import { password, publisherURL } from './env';

// Runs after reading.spec.ts: the suite shares one database and one worker, in
// file-name order, and this journey opens Entries — which would clear the
// unread state that journey asserts on.
test('the reader switches views, keeps the choice, and is offered a tab when a publisher refuses the frame', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByLabel('Feed or site address')).toBeVisible();

  await page.getByLabel('Feed or site address').fill(publisherURL);
  await page.getByRole('button', { name: 'Subscribe' }).click();
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // Newest first: Wheels, which forbids framing, then Fire, which allows it.
  const entries = page.getByTestId('entry');
  await entries.nth(1).click();
  await expect(page.getByTestId('reading-pane')).toBeVisible();
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // Reader View is the publisher's own text, not the summary the Feed carried.
  await page.getByTestId('view-reader').click();
  await expect(page.getByTestId('entry-content')).toContainText(
    "The publisher's own paragraph about fire, and how to keep it",
  );
  await expect(page.getByTestId('entry-content')).not.toContainText(
    'Site navigation the reader does not want',
  );

  // Original View embeds the live page for a publisher that allows it.
  await page.getByTestId('view-original').click();
  const frame = page.getByTestId('original-view');
  await expect(frame).toBeVisible();
  await expect(frame).toHaveAttribute('src', `${publisherURL}/fire`);
  const embedded = page.frameLocator('[data-testid="original-view"]');
  await expect(embedded.locator('nav')).toContainText(
    'Site navigation the reader does not want',
  );
  await expect(embedded.locator('article')).toHaveAttribute(
    'data-publisher-app',
    'ready',
  );
  await expect(embedded.getByText('Application error')).toHaveCount(0);

  // The publisher keeps its own origin, so its storage-dependent application
  // works. That origin still reaches neither Reader's session nor its DOM.
  const embeddedFrame = page.frame({ url: `${publisherURL}/fire` });
  expect(embeddedFrame).not.toBeNull();
  const isolation = await embeddedFrame!.evaluate(async () => {
    let parentDom: string;
    try {
      parentDom = String(!!window.parent.document.body);
    } catch {
      parentDom = 'refused';
    }
    const sessionResponse = await fetch('/session-check');
    const sessionCheck = (await sessionResponse.json()) as { cookie: string };
    return {
      origin: window.origin,
      cookies: document.cookie,
      storage: localStorage.getItem('original-view-check'),
      parentDom,
      requestCookie: sessionCheck.cookie,
    };
  });
  expect(isolation.origin).toBe(publisherURL);
  expect(isolation.cookies).not.toContain('reader_session');
  expect(isolation.storage).toBe('ready');
  expect(isolation.parentDom).toBe('refused');
  expect(isolation.requestCookie).not.toContain('reader_session');

  // The choice is the reader's, not the Entry's: the next Entry opens in it.
  await page.keyboard.press('k');
  await expect(page.getByTestId('reading-pane')).toBeVisible();
  await expect(page.getByTestId('original-view')).toHaveCount(0);
  await expect(page.getByTestId('original-view-forbidden')).toBeVisible();
  await expect(page.getByTestId('entry-content')).toContainText(
    'does not allow their page to be shown inside another site',
  );
  const newTab = page.getByTestId('open-in-new-tab');
  await expect(newTab).toHaveAttribute('href', `${publisherURL}/wheels`);
  await expect(newTab).toHaveAttribute('target', '_blank');

  // And it survives the browser: the preference is on the server.
  await page.reload();
  await expect(page.getByTestId('entry')).toHaveCount(2);
  await page.getByTestId('entry').nth(1).click();
  await expect(page.getByTestId('original-view')).toBeVisible();

  // Back to the Feed's own text, which is where the reader started.
  await page.getByTestId('view-feed').click();
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // A different port is still the same cookie host. Even if the API calls it
  // embeddable, Reader refuses the frame rather than exposing its session.
  const reader = new URL(page.url());
  const sameHostURL = `${reader.protocol}//${reader.hostname}:65534/`;
  await page.route('**/api/entries/*/original', async (route) => {
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        original: { url: sameHostURL, embeddable: true },
      }),
    });
  });
  // Moving off the Entry and back drops what was loaded for it, so the
  // Original is fetched again — this time through the route above. The Reading
  // Pane is furniture now: it never unmounts, so nothing else would.
  await page.getByTestId('entry').nth(0).click();
  await page.getByTestId('entry').nth(1).click();
  await page.getByTestId('view-original').click();
  await expect(page.getByTestId('original-view')).toHaveCount(0);
  await expect(page.getByTestId('original-view-unsafe')).toBeVisible();
  await expect(page.getByTestId('entry-content')).toContainText(
    "shares Reader's host",
  );
  await page.unroute('**/api/entries/*/original');
  await page.getByTestId('view-feed').click();
});
