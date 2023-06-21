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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTitle() = \"%v\", want \"%v\"", got, tt.want)
			}
		})
	}
}
