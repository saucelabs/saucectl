context('Assertions', () => {
  beforeEach(() => {
    cy.visit('https://example.cypress.io/commands/assertions')
  })
  it('.and() - chain multiple assertions together', () => {
    cy.get('.assertions-link')
    .should('have.class', 'active')
    .and('have.attr', 'href')
    .and('include', 'cypress.io')
  })
})
