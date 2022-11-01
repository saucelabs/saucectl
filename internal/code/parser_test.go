package code

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []TestCase
	}{
		{
			name: "basic test case match",
			input: `
context('Actions', () => {
  beforeEach(() => {
    cy.visit('https://example.cypress.io/commands/actions')
  })
  it('.type() - type into a DOM element', () => {
    // https://on.cypress.io/type
    cy.get('.action-email')
        .type('fake@email.com').should('have.value', 'fake@email.com')
  })
})
`,
			want: []TestCase {
				{
					Title: ".type() - type into a DOM element",
					Tags: "",
				},
			},
		},
		{
			name: "parse test case with multiple tags",
			input: `
context('Actions', () => {
  beforeEach(() => {
    cy.visit('https://example.cypress.io/commands/actions')
  })
  it('.type() - type into a DOM element', { tags: ['@tag1', "@tag2"] }, () => {
    // https://on.cypress.io/type
    cy.get('.action-email')
        .type('fake@email.com').should('have.value', 'fake@email.com')
  })
})
`,
			want: []TestCase {
				{
					Title: ".type() - type into a DOM element",
					Tags: "@tag1 @tag2",
				},
			},
		},
		{
			name: "parse test case with single tag",
			input: `
context('Actions', () => {
  beforeEach(() => {
    cy.visit('https://example.cypress.io/commands/actions')
  })
  it('.type() - type into a DOM element', { tags: '@tag1' }, () => {
    // https://on.cypress.io/type
    cy.get('.action-email')
        .type('fake@email.com').should('have.value', 'fake@email.com')
  })
})
`,
			want: []TestCase {
				{
					Title: ".type() - type into a DOM element",
					Tags: "@tag1",
				},
			},
		},
		{
			name: "parse test case with complex test object",
			input: `
context('Actions', () => {
  beforeEach(() => {
    cy.visit('https://example.cypress.io/commands/actions')
  })
  it('.type() - type into a DOM element', {
    tags: [
      '@tag1', 
      '@tag2'
    ],
  }, (() => {
    // https://on.cypress.io/type
    cy.get('.action-email')
        .type('fake@email.com').should('have.value', 'fake@email.com')
  })
})
`,
			want: []TestCase {
				{
					Title: ".type() - type into a DOM element",
					Tags: "@tag1 @tag2",
				},
			},
		},
		{
			name: "parse multiple test cases",
			input: `
context('Actions', function () {
  beforeEach(function () {
    cy.visit('https://example.cypress.io/commands/actions');
  });

  // https://on.cypress.io/interacting-with-elements

  it('.type() - type into a DOM element',
    {
      tags: [
        '@tag1',
        '@tag2',
      ],
    },
    function () {
      // https://on.cypress.io/type
      cy.get('.action-email')
        .type('fake@email.com').should('have.value', 'fake@email.com')

        // .type() with special character sequences
        .type('{leftarrow}{rightarrow}{uparrow}{downarrow}')
        .type('{del}{selectall}{backspace}')

        // .type() with key modifiers
        .type('{alt}{option}') //these are equivalent
        .type('{ctrl}{control}') //these are equivalent
        .type('{meta}{command}{cmd}') //these are equivalent
        .type('{shift}')

        // Delay each keypress by 0.1 sec
        .type('slow.typing@email.com', { delay: 100 })
        .should('have.value', 'slow.typing@email.com');

      cy.get('.action-disabled')
        // Ignore error checking prior to type
        // like whether the input is visible or disabled
        .type('disabled error checking', { force: true })
        .should('have.value', 'disabled error checking');
    }
  );

  it('.focus() - focus on a DOM element',
    {
      tags: '@tag1',
      otherAttr: 'somevalue',
      object: {
        hoo: 'hah',
      },
    },
    function () {
    // https://on.cypress.io/focus
      cy.get('.action-focus').focus()
        .should('have.class', 'focus')
        .prev().should('have.attr', 'style', 'color: orange;');
    }
  );
`,
			want: []TestCase {
				{
					Title: ".type() - type into a DOM element",
					Tags: "@tag1 @tag2",
				},
				{
					Title: ".focus() - focus on a DOM element",
					Tags: "@tag1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTitle() = \"%v\", want \"%v\"", got, tt.want)
			}
		})
	}
}
