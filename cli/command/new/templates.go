package new

var configTpl = `apiVersion: v1alpha
metadata:
  name: Feature XYZ
  tags:
    - e2e
    - release team
    - other tag
  build: Release $CI_COMMIT_SHORT_SHA
files:
  - {{ .TestsFolder }}/example.test.js
suites:
  - name: "saucy test"
    match: "{{ .Match }}"
    capabilities:
      browserName: "chromium"
image:
  base: {{ .Name }}
  version: {{ .Version }}
sauce:
  region: {{ .Region }}
`

// SetupTemplate describes a template for a setup
type SetupTemplate struct {
	Filename string
	Code     string
}

var testTpl = map[string]SetupTemplate{
	"testcafe": {
		"example.test.js",
		"import { Selector } from 'testcafe';\n" +
			"fixture `Getting Started`\n" +
			"	.page `http://devexpress.github.io/testcafe/example`\n\n" +
			`const testName = 'testcafe test'
test(testName, async t => {
	await t
		.typeText('#developer-name', 'devx')
		.click('#submit-button')
		.expect(Selector('#article-header').innerText).eql('Thank you, devx!');
});
	`},
	"puppeteer": {
		"example.test.js",
		`describe('saucectl demo test', () => {
	test('should verify title of the page', async () => {
		const page = (await browser.pages())[0]
		await page.goto('https://www.saucedemo.com/');
		expect(await page.title()).toBe('Swag Labs');
	});
});
`},
	"playwright": {
		"example.test.js",
		`describe('saucectl demo test', () => {
	test('should verify title of the page', async () => {
		await page.goto('https://www.saucedemo.com/');
		expect(await page.title()).toBe('Swag Labs');
	});
});
`},
	"cypress": {
		"example.test.js",
		`context('Actions', () => {
		beforeEach(() => {
			cy.visit('https://example.cypress.io/commands/actions')
		})
		it('.type() - type into a DOM element', () => {
			// https://on.cypress.io/type
			cy.get('.action-email')
				.type('fake@email.com').should('have.value', 'fake@email.com')
		})
	})`},
}
