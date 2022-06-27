context('Traversal', () => {
  beforeEach(() => {
    cy.visit('https://example.cypress.io/commands/traversal')
  })
  it('.children() - get children of DOM elements', () => {
    cy.get('.traversal-breadcrumb')
    .children('.active')
    .should('contain', 'Data')
  })
})
