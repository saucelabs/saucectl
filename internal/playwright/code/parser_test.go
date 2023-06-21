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
test.describe('New Todo', () => {
  test('should allow me to add todo items', async ({ page }) => {
  });

  test('should allow me to add todo items', async ({ page }) => {
  });
});
`,
			want: []TestCase{
				{
					Title: "New Todo",
				},
				{
					Title: "should allow me to add todo items",
				},
				{
					Title: "should allow me to add todo items",
				},
			},
		},

		{
			name: "complex case match",
			input: `
// @ts-check
const { test, expect } = require('@playwright/test');

test.beforeEach(async ({ page }) => {
  await page.goto('https://demo.playwright.dev/todomvc');
});

const TODO_ITEMS = [
  'buy some cheese',
  'feed the cat',
  'book a doctors appointment'
];

test.describe('New Todo', async () => {
  test('should allow me to add todo items', async ({ page }) => {
    // Create 1st todo.
    await page.locator('.new-todo').fill(TODO_ITEMS[0]);
    await page.locator('.new-todo').press('Enter');

    // Make sure the list only has one todo item.
    await expect(page.locator('.view label')).toHaveText([
      TODO_ITEMS[0]
    ]);

    // Create 2nd todo.
    await page.locator('.new-todo').fill(TODO_ITEMS[1]);
    await page.locator('.new-todo').press('Enter');

    // Make sure the list now has two todo items.
    await expect(page.locator('.view label')).toHaveText([
      TODO_ITEMS[0],
      TODO_ITEMS[1]
    ]);

    await checkNumberOfTodosInLocalStorage(page, 2);
  });


  test('Non-empty', async ({ page }) => {
    // Create 1st todo.
  });

  test('should clear text input field when an item is added @fast', async ({ page }) => {
    // Create one todo item.
    await page.locator('.new-todo').fill(TODO_ITEMS[0]);
    await page.locator('.new-todo').press('Enter');

    // Check that input is empty.
    await expect(page.locator('.new-todo')).toBeEmpty();
    await checkNumberOfTodosInLocalStorage(page, 1);
  });
`,
			want: []TestCase{
				{
					Title: "New Todo",
				},
				{
					Title: "should allow me to add todo items",
				},
				{
					Title: "Non-empty",
				},
				{
					Title: "should clear text input field when an item is added @fast",
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
