package grep

import (
	"testing"
	"testing/fstest"
)

func Test_matchFS(t *testing.T) {
	mockFS := fstest.MapFS{
		"spec1.js": {
			Data: []byte(`
context('Actions', () => {
  beforeEach(() => {
    cy.visit('https://example.cypress.io/commands/actions')
  })
  it('.type() - type into a DOM element', { tags: ['@smoke'] }, () => {
    // https://on.cypress.io/type
    cy.get('.action-email')
        .type('fake@email.com').should('have.value', 'fake@email.com')
  })

  it('.focus() - focus on a DOM element', { tags: '@flakey' }, () => {
    // https://on.cypress.io/focus
      cy.get('.action-focus').focus()
        .should('have.class', 'focus')
        .prev().should('have.attr', 'style', 'color: orange;');
    }
  );
})
`),
		},
		"spec2.js": {
			Data: []byte(`
context('Assertions', () => {
  beforeEach(() => {
    cy.visit('https://example.cypress.io/commands/assertions')
  })
  it('.and() - chain multiple assertions together', {tags: ['@smoke', '@flakey']}, () => {
    cy.get('.assertions-link')
    .should('have.class', 'active')
    .and('have.attr', 'href')
    .and('include', 'cypress.io')
  })
})
`),
		},
	}

	m, u := Match(mockFS, []string{"spec1.js", "spec2.js"}, "", "@flakey")

	got := len(m) + len(u)
	want := len(mockFS)
	if (got != want) {
		t.Errorf("The returned slices from Match should not have duplicate values: got(%d) want(%d)", got, want)
	}
 }
