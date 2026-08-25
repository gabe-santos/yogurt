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
  await expect(page.getByTestId('entry-drawer')).toBeVisible();
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

  // The embed is isolated from this app: an opaque origin, so it reaches
  // neither the session cookie that fetched it nor the DOM around it.
  const embeddedFrame = page.frame({ url: `${publisherURL}/fire` });
  expect(embeddedFrame).not.toBeNull();
  const isolation = await embeddedFrame!.evaluate(() => {
    let cookies: string;
    try {
      cookies = document.cookie;
    } catch {
      cookies = 'refused';
    }
    let parentDom: string;
    try {
      parentDom = String(!!window.parent.document.body);
    } catch {
      parentDom = 'refused';
    }
    return { origin: window.origin, cookies, parentDom };
  });
  expect(isolation.origin).toBe('null');
  expect(isolation.cookies).not.toContain('session');
  expect(isolation.parentDom).toBe('refused');

  // The choice is the reader's, not the Entry's: the next Entry opens in it.
  await page.keyboard.press('k');
  await expect(page.getByTestId('entry-drawer')).toBeVisible();
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
  await page.keyboard.press('Escape');
});
