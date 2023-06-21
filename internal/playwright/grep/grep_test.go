package grep

import (
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/maps"
)

func TestMatchFiles(t *testing.T) {
	type testCase struct {
		name          string
		grep          string
		grepInvert    string
		wantMatched   []string
		wantUnmatched []string
	}

	mockFS := fstest.MapFS{
		"demo-todo.spec.js": {
			Data: []byte(`
test.describe('New Todo', () => {
  test('should allow me to add todo items @query', async ({ page }) => {
  });

  test('should allow me to add multiple todo items @save', async ({ page }) => {
  });
});
`),
		},
		"demo-step.spec.js": {
			Data: []byte(`
test.describe('New Step', () => {
  test('should allow me to add one step @fast @unique', async ({ page }) => {
  });

  test('should allow me to add multiple steps @slow @unique', async ({ page }) => {
  });
});
`),
		},
	}

	testCases := []testCase{
		{
			name:          "Base",
			grep:          "New Todo",
			wantMatched:   []string{"demo-todo.spec.js"},
			wantUnmatched: []string{"demo-step.spec.js"},
		},
		{
			name:          "tag",
			grep:          "@fast",
			wantMatched:   []string{"demo-step.spec.js"},
			wantUnmatched: []string{"demo-todo.spec.js"},
		},
		{
			name:          "filename",
			grep:          "demo-step",
			wantMatched:   []string{"demo-step.spec.js"},
			wantUnmatched: []string{"demo-todo.spec.js"},
		},
		{
			name:          "tags same spec",
			grep:          "@fast|@slow",
			wantMatched:   []string{"demo-step.spec.js"},
			wantUnmatched: []string{"demo-todo.spec.js"},
		},
		{
			name:          "tags across specs",
			grep:          "@fast|@save",
			wantMatched:   []string{"demo-todo.spec.js", "demo-step.spec.js"},
			wantUnmatched: []string(nil),
		},
		{
			name: "combined tag - not found",
			// Note: example pattern should be "(?=.*@fast)(?=.*@slow)"
			// But Go RegExp library does not support the Perl Look Around.
			grep:          "(.*@fast)(.*)(.*@slow)",
			wantMatched:   []string(nil),
			wantUnmatched: []string{"demo-todo.spec.js", "demo-step.spec.js"},
		},
		{
			name: "combined tag",
			// Note: example pattern should be "(?=.*@fast)(?=.*@unique)"
			// But Go RegExp library does not support the Perl Look Around.
			grep:          "(.*@fast)(.*)(.*@unique)",
			wantMatched:   []string{"demo-step.spec.js"},
			wantUnmatched: []string{"demo-todo.spec.js"},
		},
		{
			name:          "grepInvert",
			grep:          "New Todo",
			grepInvert:    "demo-todo",
			wantMatched:   []string(nil),
			wantUnmatched: []string{"demo-todo.spec.js", "demo-step.spec.js"},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			matched, unmatched := MatchFiles(mockFS, maps.Keys(mockFS), tt.grep, tt.grepInvert)

			if len(mockFS) != len(unmatched)+len(matched) {
				t.Errorf("MatchFiles: unexpected lenght of results. got: %d, want: %d", len(unmatched)+len(matched), len(mockFS))

			}
			if diff := cmp.Diff(matched, tt.wantMatched); diff != "" {
				t.Errorf("MatchFiles: difference in matched: %s", diff)
			}
			if diff := cmp.Diff(unmatched, tt.wantUnmatched); diff != "" {
				t.Errorf("MatchFiles: difference in unmatched: %s", diff)
			}

		})
	}
}
