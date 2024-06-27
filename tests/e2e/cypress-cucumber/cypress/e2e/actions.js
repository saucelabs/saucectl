import { When, Then } from "@badeball/cypress-cucumber-preprocessor";

When('I open cypress actions page', () => {
  cy.visit('https://example.cypress.io/commands/actions');
});

Then(`I enter {string} as the email address`, (email) => {
  cy.get('.action-email')
      .type(email).should('have.value', email);
});
