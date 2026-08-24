import { expect, test } from '@playwright/test';

import { password, publisherURL } from './env';

test('the reader opens an Entry, reads it, and triages by keyboard', async ({ page }) => {
	await page.goto('/login');
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page.getByLabel('Feed or site address')).toBeVisible();

	await page.getByLabel('Feed or site address').fill(publisherURL);
	await page.getByRole('button', { name: 'Subscribe' }).click();
	await expect(page.getByTestId('entry')).toHaveCount(2);

	// Newest first: Wheels, then Fire.
	const entries = page.getByTestId('entry');
	await expect(entries.nth(0)).toContainText('unread');
	await expect(entries.nth(1)).toContainText('unread');

	// Opening an Entry shows the Feed-supplied content, sanitised, and marks it
	// Read by default.
	await entries.nth(0).click();
	const drawer = page.getByTestId('entry-drawer');
	await expect(drawer).toBeVisible();
	await expect(page.getByTestId('entry-content')).toContainText('Round, and it rolls.');
	await expect(entries.nth(0)).not.toContainText('unread');

	// Next/previous from the keyboard, without leaving the drawer.
	await page.keyboard.press('j');
	await expect(page.getByTestId('entry-content')).toContainText('Keeping a fire alive overnight.');
	await expect(entries.nth(1)).not.toContainText('unread');

	await page.keyboard.press('k');
	await expect(page.getByTestId('entry-content')).toContainText('Round, and it rolls.');

	// Manual unread always overrides mark-on-open — including after navigating
	// away and back, which would otherwise re-trigger mark-on-open.
	await page.keyboard.press('m');
	await expect(entries.nth(0)).toContainText('unread');
	await page.keyboard.press('j');
	await page.keyboard.press('k');
	await expect(entries.nth(0)).toContainText('unread');

	// Close from the keyboard.
	await page.keyboard.press('Escape');
	await expect(drawer).toBeHidden();

	// The Unread filter scopes the list to what mark-on-open left unread.
	await page.getByTestId('filter-unread').click();
	await expect(page.getByTestId('entry')).toHaveCount(1);
	await expect(page.getByTestId('entry').first()).toContainText('Wheels: a review');

	await page.getByTestId('filter-all').click();
	await expect(page.getByTestId('entry')).toHaveCount(2);

	// The `?` help dialog lists every binding from the one table.
	await page.keyboard.press('?');
	const help = page.getByTestId('help-dialog');
	await expect(help).toBeVisible();
	await expect(help).toContainText('Next Entry');
	await page.keyboard.press('Escape');
	await expect(help).toBeHidden();

	// Enter on a focused button is native activation, not the global Enter
	// binding: it must not silently open whatever the drawer last showed.
	await page.getByTestId('filter-unread').focus();
	await page.keyboard.press('Enter');
	await expect(drawer).toBeHidden();
	await expect(page.getByTestId('entry')).toHaveCount(1);
	await page.getByTestId('filter-all').click();
});
