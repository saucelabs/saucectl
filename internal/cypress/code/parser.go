package code

import (
	"regexp"
	"strings"
)

// Functions to parse cypress spec files for testcases and their metadata (e.g.
// test titles and tags).
// The general strategy is to parse in multiple passes; the first pass extracts
// testcase definitions (e.g. it('some test title', { tags: '@tag' }, () => ...)
// and subsequent passes extract the titles and tags.

var (
	reTestCasePattern  = regexp.MustCompile(`(?m)^ *(?:it|test)(?:\.\w+)?(\([\s\S]*?,\s*(?:function)?\s*\()`)
	reTitlePattern     = regexp.MustCompile(`\(["'\x60](.*?)["'\x60],\s*?(?:function)?|[{(]`)
	reMultiTagPattern  = regexp.MustCompile(`tags\s*:\s*\[([\s\S]*?)\]`)
	reSingleTagPattern = regexp.MustCompile(`tags\s*:\s*['"](.*?)["']`)
)

// TestCase describes the metadata for a cypress test case parsed from a cypress spec file.
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
