const { test, expect } = require('@playwright/test');

test('should verify title of the page', async ({ page }) => {
    await page.goto('https://playwright.dev/');
    await expect(page).toHaveTitle(/Playwright/);
});
