const { it, expect } = require('@playwright/test');

it('should verify title of the page', async ({ page }) => {
	await page.goto('https://www.saucedemo.com/');
	expect(await page.title()).toBe('Swag Labs');
});