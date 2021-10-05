const { test, expect } = require('@playwright/test');

test('should verify title of the page', async ({ page }) => {
    await page.goto('https://google.com/');
    await expect(page).toHaveTitle(/Google/);
});
