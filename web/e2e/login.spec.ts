import { expect, test } from '@playwright/test';

import { password } from './env';

test('the reader signs in, stays signed in across a reload, and signs out', async ({ page }) => {
	await page.goto('/');
	await expect(page).toHaveURL(/\/login$/);

	await page.getByLabel('Password').fill('wrong password');
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page.getByRole('alert')).toHaveText('incorrect password');

	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page.getByLabel('Feed or site address')).toBeVisible();

	// The session cookie outlives the page, which is the point of it.
	await page.reload();
	await expect(page.getByLabel('Feed or site address')).toBeVisible();

	await page.getByRole('button', { name: 'Sign out' }).click();
	await expect(page).toHaveURL(/\/login$/);

	await page.goto('/');
	await expect(page).toHaveURL(/\/login$/);
});
