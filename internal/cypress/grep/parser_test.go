package grep

import "testing"

func TestParse(t *testing.T) {
	type testCase struct {
		input string
		want  bool
	}
	tests := []struct {
		name       string
		expression string
		testCases  []testCase
	}{
		{
			name:       "Simple expression",
			expression: "title",
			testCases: []testCase{
				{
					input: "a test title",
					want:  true,
				},
				{
					input: "another test",
					want:  false,
				},
			},
		},
		{
			name:       "Inverted expression",
			expression: "-title",
			testCases: []testCase{
				{
					input: "a test title",
					want:  false,
				},
				{
					input: "another test",
					want:  true,
				},
			},
		},
		{
			name:       "OR matching",
			expression: "title; test",
			testCases: []testCase{
				{
					input: "a test title",
					want:  true,
				},
				{
					input: "another test",
					want:  true,
				},
				{
					input: "should not match",
					want:  false,
				},
			},
		},
		{
			name:       "complex matching",
			expression: "test; -title",
			testCases: []testCase{
				{
					input: "a test title",
					want:  false,
				},
				{
					input: "another test",
					want:  true,
				},
				{
					input: "should match this test",
					want:  true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parse(tt.expression)
			for _, tc := range tt.testCases {
				if got := p.Match(tc.input); got != tc.want {
					t.Errorf("expression \"%s\" should match \"%s\"", tt.expression, tc.input)
				}
			}
		})
	}
}
