package grep

import "testing"

// Most test cases copied from cypress-grep unit tests: https://github.com/cypress-io/cypress/blob/d422aadfa10e5aaac17ed0e4dd5e18a73d821490/npm/grep/cypress/e2e/unit.js
func TestParseGrepTagsExp(t *testing.T) {
	type testCase struct {
		exp  string
		tags string
		want bool
	}
	tests := []struct {
		name      string
		testCases []testCase
	}{
		{
			name: "Simple expression",
			testCases: []testCase{
				{
					exp:  "@tag",
					tags: "@tag",
					want: true,
				},
				{
					exp:  "@tag",
					tags: "@tag1",
					want: false,
				},
				{
					exp:  "@tag",
					tags: "",
					want: false,
				},
			},
		},
		{
			name: "AND matching",
			testCases: []testCase{
				{
					exp:  "smoke+slow",
					tags: "fast smoke",
					want: false,
				},
				{
					exp:  "smoke+slow",
					tags: "mobile smoke slow",
					want: true,
				},
				{
					exp:  "smoke+slow",
					tags: "slow extra smoke",
					want: true,
				},
				{
					exp:  "smoke+slow",
					tags: "smoke",
					want: false,
				},
				{
					exp:  "@smoke+@screen-b",
					tags: "@smoke @screen-b",
					want: true,
				},
			},
		},
		{
			name: "OR matching",
			testCases: []testCase{
				{
					exp:  "smoke slow",
					tags: "fast smoke",
					want: true,
				},
				{
					exp:  "smoke",
					tags: "mobile smoke slow",
					want: true,
				},
				{
					exp:  "slow",
					tags: "slow extra smoke",
					want: true,
				},
				{
					exp:  "smoke",
					tags: "smoke",
					want: true,
				},
				{
					exp:  "smoke",
					tags: "slow",
					want: false,
				},
				{
					exp:  "@smoke,@slow",
					tags: "@fast @smoke",
					want: true,
				},
			},
		},
		{
			name: "inverted tag",
			testCases: []testCase{
				{
					exp:  "smoke+-slow",
					tags: "smoke slow",
					want: false,
				},
				{
					exp:  "mobile+-slow",
					tags: "smoke slow",
					want: false,
				},
				{
					exp:  "smoke -slow",
					tags: "smoke fast",
					want: true,
				},
				{
					exp:  "-slow",
					tags: "smoke slow",
					want: false,
				},
				{
					exp:  "-slow",
					tags: "smoke",
					want: true,
				},
				{
					exp:  "-slow",
					tags: "",
					want: true,
				},
			},
		},
		{
			name: "global inverted tag",
			testCases: []testCase{
				{
					// This expression is equivalent to @smoke+-@slow @e2e+-@slow
					exp:  "@smoke @e2e --@slow",
					tags: "@smoke @slow",
					want: false,
				},
				{
					exp:  "@smoke @e2e --@slow",
					tags: "@smoke",
					want: true,
				},
				{
					exp:  "@smoke @e2e --@slow",
					tags: "@slow",
					want: false,
				},
			},
		},
		{
			name: "empty values",
			testCases: []testCase{
				{
					exp:  "",
					tags: "@smoke @slow",
					want: true,
				},
				{
					exp:  "",
					tags: "",
					want: true,
				},
				{
					exp:  "@smoke",
					tags: "",
					want: false,
				},
			},
		},
		{
			name: "should handle slightly malformed expressions",
			testCases: []testCase{
				{
					exp:  "    +@smoke",
					tags: "@smoke @slow",
					want: true,
				},
				{
					exp:  "    +@smoke",
					tags: "@slow",
					want: false,
				},
				{
					exp:  ",, @tag1,-@tag2,, ,, ,",
					tags: "@tag1",
					want: true,
				},
				{
					exp:  ",, @tag1,-@tag2,, ,, ,",
					tags: "@tag2",
					want: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, tc := range tt.testCases {
				p := ParseGrepTagsExp(tc.exp)
				if got := p.Eval(tc.tags); got != tc.want {
					t.Errorf("expression (%s) for (%s) got = (%t); want = (%t)", tc.exp, tc.tags, got, tc.want)
				}
			}
		})
	}
}

func TestParseTitleGrepExp(t *testing.T) {
	type testCase struct {
		exp   string
		title string
		want  bool
	}
	tests := []struct {
		name      string
		testCases []testCase
	}{
		{
			name: "Simple tag",
			testCases: []testCase{
				{
					exp:   "@tag1",
					title: "no tag1 here",
					want:  false,
				},
				{
					title: "has @tag1 in the name",
					want:  true,
				},
			},
		},
		{
			name: "With invert title",
			testCases: []testCase{
				{
					exp:   "-hello",
					title: "no greetings",
					want:  true,
				},
				{
					exp:   "-hello",
					title: "has hello world",
					want:  false,
				},
			},
		},
		{
			name: "Multiple invert strings and a simple one",
			testCases: []testCase{
				{
					exp:   "-name;-hey;number",
					title: "number should only be matches without a n-a-m-e",
					want:  true,
				},
				{
					exp:   "-name;-hey;number",
					title: "number can't be name",
					want:  false,
				},
				{
					exp:   "-name;-hey;number",
					title: "The man needs a name",
					want:  false,
				},
				{
					exp:   "-name;-hey;number",
					title: "number hey name",
					want:  false,
				},
				{
					exp:   "-name;-hey;number",
					title: "numbers hey name",
					want:  false,
				},
				{
					exp:   "-name;-hey;number",
					title: "number hsey nsame",
					want:  true,
				},
				{
					exp:   "-name;-hey;number",
					title: "This wont match",
					want:  false,
				},
			},
		},
		{
			name: "Only inverted strings",
			testCases: []testCase{
				{
					exp:   "-name;-hey",
					title: "I'm matched",
					want:  true,
				},
				{
					exp:   "-name;-hey",
					title: "hey! I'm not",
					want:  false,
				},
				{
					exp:   "-name;-hey",
					title: "My name is weird",
					want:  false,
				},
			},
		},
		{
			name: "Empty values",
			testCases: []testCase{
				{
					exp:   "",
					title: "",
					want:  true,
				},
				{
					exp:   "",
					title: "test title",
					want:  true,
				},
				{
					exp:   "some title to match",
					title: "",
					want:  false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, tc := range tt.testCases {
				p := ParseGrepTitleExp(tc.exp)
				if got := p.Eval(tc.title); got != tc.want {
					t.Errorf("title expression (%s) for (%s) got = (%t); want = (%t)", tc.exp, tc.title, got, tc.want)
				}
			}
		})
	}
}
