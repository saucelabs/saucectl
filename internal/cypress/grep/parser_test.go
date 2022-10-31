package grep

import "testing"

func TestParseGrepTagsExp(t *testing.T) {
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
			expression: "@tag",
			testCases: []testCase{
				{
					input: "@tag",
					want:  true,
				},
				{
					input: "@tag1",
					want:  false,
				},
			},
		},
		{
			name:       "AND matching",
			expression: "@tag1+@tag2",
			testCases: []testCase{
				{
					input: "@tag1",
					want:  false,
				},
				{
					input: "@tag1 @tag2",
					want:  true,
				},
			},
		},
		{
			name:       "OR matching",
			expression: "@tag1 @tag2",
			testCases: []testCase{
				{
					input: "@tag1 @anotherTag",
					want:  true,
				},
				{
					input: "@tag2 @anotherTag",
					want:  true,
				},
				{
					input: "@anotherTag",
					want:  false,
				},
			},
		},
		{
			name:       "inverted tag",
			expression: "-@tag1",
			testCases: []testCase{
				{
					input: "@tag1 @tag2",
					want:  false,
				},
				{
					input: "@tag2",
					want:  true,
				},
			},
		},
		{
			name:       "NOT expression",
			// This expression is equivalent to @smoke+-@slow @e2e+-@slow
			expression: "@smoke @e2e --@slow",
			testCases: []testCase{
				{
					input: "@smoke @slow",
					want:  false,
				},
				{
					input: "@smoke",
					want:  true,
				},
				{
					input: "@slow",
					want:  false,
				},
			},
		},
		{
			name:       "empty expression",
			expression: "",
			testCases: []testCase{
				{
					input: "@smoke @slow",
					want:  false,
				},
				{
					input: "",
					want:  false,
				},
			},
		},
		{
			name:       "should handle slightly malformed expressions",
			expression: "    +@smoke",
			testCases: []testCase{
				{
					input: "@smoke @slow",
					want:  true,
				},
				{
					input: "",
					want:  false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParseGrepTagsExp(tt.expression)
			for _, tc := range tt.testCases {
				if got := p.Eval(tc.input); got != tc.want {
					t.Errorf("expression \"%s\" should match \"%s\"", tt.expression, tc.input)
				}
			}
		})
	}
}

func TestParseGrepExp(t *testing.T) {
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
			p := ParseGrepExp(tt.expression)
			for _, tc := range tt.testCases {
				if got := p.Eval(tc.input); got != tc.want {
					t.Errorf("expression \"%s\" should match \"%s\"", tt.expression, tc.input)
				}
			}
		})
	}
}
