package grep

import (
	"reflect"
	"testing"
	"testing/fstest"
)

func TestMatchFiles(t *testing.T) {
	mockFS := fstest.MapFS{
		"scenario1.feature": {
			Data: []byte(`
@act1
Feature: Scenario 1

        @interior @nomatch
        Scenario: Dinner scene
                When Turkey is served
                Then I say "bon appetit!"
`),
		},
		"scenario2.feature": {
			Data: []byte(`
@act3
Feature: Scenario 2

        @exterior @nomatch
        Scenario: Exterior scene
                When The character exits the house
                Then The camera pans out to show the exterior

        @interior @nomatch
        Scenario: Interior scene
                When The character enters the house
                Then The character's leitmotif starts
`),
		},
		"scenario3.feature": {
			Data: []byte(`
@act3 @credits
Feature: Scenario 3

	@nomatch
        Scenario: Epilogue
                When The credits reach mid point
                Then Start the first mid-credit scene

	@nomatch
        Scenario: Last Bonus Scene
                When The credits reach the end
                Then Start the end-credit scene
`),
		},
	}

	files := []string{
		"scenario1.feature",
		"scenario2.feature",
		"scenario3.feature",
	}

	tests := []struct {
		name          string
		files         []string
		tagExpression string
		wantMatched   []string
		wantUnmatched []string
	}{
		{
			name:          "matches a single tag",
			files:         files,
			tagExpression: "@act1",
			wantMatched: []string{
				"scenario1.feature",
			},
			wantUnmatched: []string{
				"scenario2.feature",
				"scenario3.feature",
			},
		},
		{
			name: "matches scenario tag",
			files: files,
			tagExpression: "@interior",
			wantMatched: []string{
				"scenario1.feature",
				"scenario2.feature",
			},
			wantUnmatched: []string{
				"scenario3.feature",
			},
		},
		{
			name: "matches multiple tags",
			files: files,
			tagExpression: "@act3 and @credits",
			wantMatched: []string {
				"scenario3.feature",
			},
			wantUnmatched: []string {
				"scenario1.feature",
				"scenario2.feature",
			},
		},
		{
			name: "matches multiple tags with negation",
			files: files,
			tagExpression: "@act3 and not @credits",
			wantMatched: []string {
				"scenario2.feature",
			},
			wantUnmatched: []string {
				"scenario1.feature",
				"scenario3.feature",
			},
		},
		{
			name: "no matches with negation",
			files: files,
			tagExpression: "not @nomatch",
			wantMatched: []string(nil),
			wantUnmatched: []string {
				"scenario1.feature",
				"scenario2.feature",
				"scenario3.feature",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, unmatched := MatchFiles(mockFS, tt.files, tt.tagExpression)
			if !reflect.DeepEqual(matched, tt.wantMatched) {
				t.Errorf("MatchFiles() got matched %v, want %v", matched, tt.wantMatched)
			}
			if !reflect.DeepEqual(unmatched, tt.wantUnmatched) {
				t.Errorf("MatchFiles() got unmatched %v, want %v", unmatched, tt.wantUnmatched)
			}
		})
	}
}
