var expect = require('chai').expect

describe('saucectl demo test', () => {
	test('should verify title of the page', async () => {
		const page = (await browser.pages())[0]
		await page.goto('https://www.saucedemo.com/');
		expect( await page.title()).to.equal('Swag Labs');
	});
});
