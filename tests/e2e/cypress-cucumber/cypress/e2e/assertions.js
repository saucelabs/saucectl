import { When, Then } from "@badeball/cypress-cucumber-preprocessor";

When('I open cypress assertions page', () => {
    cy.visit('https://example.cypress.io/commands/assertions');
});

Then('I should see a table', () => {
    cy.get('.assertion-table')
        .find('tbody tr:last')
        // finds first <td> element with text content matching regular expression
        .contains('td', /column content/i)
        .should('be.visible')
});
