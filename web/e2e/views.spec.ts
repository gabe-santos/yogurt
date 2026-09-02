import { expect, test } from '@playwright/test';

import { password, publisherURL } from './env';

// Runs after reading.spec.ts: the suite shares one database and one worker, in
// file-name order, and this journey reuses the Feed and Entries that journey
// already subscribed to rather than subscribing again, which the app would
// refuse.
test('the reader switches views, keeps the choice, and is offered a tab when a publisher refuses the frame', async ({
  page,
}) => {
  await page.goto('/login');
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByTestId('entry')).toHaveCount(2);

  // Newest first: Wheels, which forbids framing, then Fire, which allows it.
  const entries = page.getByTestId('entry');
  await entries.nth(1).click();
  await expect(page.getByTestId('reading-pane')).toBeVisible();
  await expect(page.getByTestId('entry-content')).toContainText(
    'Keeping a fire alive overnight.',
  );

  // Feed View nests under the page's single h1 and the Entry's own h2 —
  // never a second h1, even though this Feed's own description carries a
  // heading of its own: it demotes to h3, so the page's outline stays
  // exactly one h1 and one h2.
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('h2')).toHaveCount(1);
  await expect(page.locator('h2')).toContainText('Fire, and how to keep it');
  await expect(page.getByTestId('entry-content').locator('h3')).toHaveText(
    'Keeping the flame',
  );

  // The travelling surface remains an immediate state marker when readers ask
  // for reduced motion; its geometry is already that of the chosen cell when
  // the click completes.
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.getByTestId('view-reader').click();
  await expect(page.getByTestId('view-reader')).toHaveAttribute(
    'data-state',
    'active',
  );
  const reducedMotionIndicatorIsAligned = await page
    .getByRole('tablist', { name: 'View' })
    .evaluate((list) => {
      const indicator = list.parentElement?.firstElementChild;
      const active = list.querySelector<HTMLElement>('[data-state="active"]');
      if (!indicator || !active) return false;

      const indicatorRect = indicator.getBoundingClientRect();
      const activeRect = active.getBoundingClientRect();
      return (
        Math.abs(indicatorRect.left - activeRect.left) < 0.01 &&
        Math.abs(indicatorRect.top - activeRect.top) < 0.01 &&
        Math.abs(indicatorRect.width - activeRect.width) < 0.01 &&
        Math.abs(indicatorRect.height - activeRect.height) < 0.01
      );
    });
  expect(reducedMotionIndicatorIsAligned).toBe(true);

  // Reader View is the publisher's own text, not the summary the Feed carried.
  await expect(page.getByTestId('entry-content')).toContainText(
    "The publisher's own paragraph about fire, and how to keep it",
  );
  await expect(page.getByTestId('entry-content')).not.toContainText(
    'Site navigation the reader does not want',
  );

  // Reader View, including its own extracted markup, still nests under one
  // h1 and one h2. See issue #37.
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('h2')).toHaveCount(1);

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

  // Original View embeds the publisher's page in an iframe with its own
  // document; the app page keeps its own single h1 and h2 regardless. See
  // issue #37.
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('h2')).toHaveCount(1);

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

  // The face an Entry is read in is the reader's too, and it outlives the
  // reload the same way the view does.
  await page.getByTestId('reading-font-serif').click();
  await expect(page.getByTestId('reading-prose')).toHaveCSS(
    'font-family',
    /Literata/,
  );
  await page.reload();
  await expect(page.getByTestId('reading-prose')).toHaveCSS(
    'font-family',
    /Literata/,
  );

  // Extracted Articles are full of `<em>`, and a face whose italic is not
  // loaded gets one the browser slants by hand. Both reading faces ship a
  // drawn italic, and `load` resolves with nothing at all when the stylesheet
  // declaring one is missing.
  const drawnItalics = await page.evaluate(async () => {
    const [sans, serif] = await Promise.all([
      document.fonts.load("italic 18px 'Geist Variable'"),
      document.fonts.load("italic 18px 'Literata Variable'"),
    ]);
    return { sans: sans.length, serif: serif.length };
  });
  expect(drawnItalics.sans).toBeGreaterThan(0);
  expect(drawnItalics.serif).toBeGreaterThan(0);

  await page.getByTestId('reading-font-sans').click();
  await expect(page.getByTestId('reading-prose')).toHaveCSS(
    'font-family',
    /Geist/,
  );
});
