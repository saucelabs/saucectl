package code

import (
	"regexp"
	"strings"
)

var (
	reTestCasePattern  = regexp.MustCompile(`(?m)^ *(?:it|test)(?:\.\w+)?\(([\s\S]*?,\s*\()`)
	reTitlePattern     = regexp.MustCompile(`["'\x60](.*)["'\x60], *[{(]`)
	reMultiTagPattern  = regexp.MustCompile(`tags\s*:\s*\[([\s\S]*?)\]`)
	reSingleTagPattern = regexp.MustCompile(`tags\s*:\s*['"](.*?)["']`)
)

// TestCase describes a cypress test case parsed from a cypress spec file.
type TestCase struct {
	// Title is the name of the test case, the first argument to `it|test`.
	Title string
	// Tags is an optional list of tags. This is simply a space delimited
	// concatenation of all tags defined for the testcase.
	Tags  string
}

// Parse takes the contents of a test file and parses testcases
func Parse(input string) []TestCase {
	matches := reTestCasePattern.FindAllStringSubmatch(input, -1)

	var testCases []TestCase
	for _, m := range matches {
		tc := TestCase{}
		argSubMatch := m[1]

		tc.Title = parseTitle(argSubMatch)
		tc.Tags = parseTags(argSubMatch)

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

func parseTags(input string) string {
	var tags []string
	tagMatch := reSingleTagPattern.FindStringSubmatch(input)
	if tagMatch == nil {
		tagMatch = reMultiTagPattern.FindStringSubmatch(input)
	}
	if tagMatch != nil {
		rawTags := strings.Split(tagMatch[1], ",")
		for _, t := range rawTags {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, strings.Trim(t, `"'`))
			}
		}
		return strings.Join(tags, " ")
	}
	return ""
}
