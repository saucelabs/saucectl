package new

var configTpl = `
apiVersion: v1
metadata:
  name: Feature XYZ
  tags:
    - e2e
    - release team
    - other tag
  build: Release $CI_COMMIT_SHORT_SHA
files:
  - ./tests/example.test.js
image:
  base: saucelabs/sauce-{{ .Framework }}-runner
  version: latest
`

// SetupTemplate describes a template for a setup
type SetupTemplate struct {
	Filename string
	Code     string
}

var testTpl = map[string]SetupTemplate{
	"puppeteer": {
		"example.test.js",
		`describe('Herokuapp login page is constructed correctly', () => {
	test('Page is available', async () => {
		const page = (await browser.pages())[0]
		await page.goto('https://www.saucedemo.com/');
		expect(await page.title()).toBe('Swag Labs');
	});
});
`},
	"playwright": {
		"example.test.js",
		`describe('Herokuapp login page is constructed correctly', () => {
	test('Page is available', async () => {
		await page.goto('https://www.saucedemo.com/');
		expect(await page.title()).toBe('Swag Labs');
	});
});
`},
}
