package code

import (
	"regexp"
)

// Functions to parse playwright spec files for testcases.
// The general strategy is to parse in multiple passes; the first pass extracts
// testcase definitions (e.g. it('some test title', { tags: '@tag' }, () => ...)
// and subsequent passes extract the titles.

var (
	reTestCasePattern = regexp.MustCompile(`(?m)^ *test(?:\.describe)?(?:\.\w+)?(\([\s\S]*?,\s*(?:async)?\s*(?:function)?\s*\()`)
	reTitlePattern    = regexp.MustCompile(`\(["'\x60](.*?)["'\x60],\s*?(?:function)?|[{(]`)
)

// TestCase describes the metadata for a playwright test case parsed from a playwright spec file.
type TestCase struct {
	// Title is the name of the test case, the first argument to `test(.describe)`.
	Title string
}

// Parse takes the contents of a test file and parses testcases
func Parse(input string) []TestCase {
	matches := reTestCasePattern.FindAllStringSubmatch(input, -1)

	var testCases []TestCase
	for _, m := range matches {
		tc := TestCase{}
		argSubMatch := m[1]

		tc.Title = parseTitle(argSubMatch)

		testCases = append(testCases, tc)
	}
	return testCases
}

func parseTitle(input string) string {
	titleMatch := reTitlePattern.FindStringSubmatch(input)
	if titleMatch != nil {
		return titleMatch[1]
	}

	return ""
}
