const { it, expect } = require('@playwright/test');

it('should verify title of the page', async ({ page }) => {
	await page.goto('https://google.com');
  await expect(page).toHaveTitle(/Google/);
});
