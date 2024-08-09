// Package tag defines functions to parse cucumber feature files and filter them by cucumber tag expressions
package tag

import (
	"io/fs"

	gherkin "github.com/cucumber/gherkin/go/v28"
	messages "github.com/cucumber/messages/go/v24"
	tagexpressions "github.com/cucumber/tag-expressions/go/v6"
	"github.com/rs/zerolog/log"
)

// MatchFiles finds feature files that include scenarios with tags that match the given tag expression.
// A tag expression is a simple boolean expression including the logical operators "and", "or", "not".
func MatchFiles(sys fs.FS, files []string, tagExpression string) (matched []string, unmatched []string) {
	tagMatcher, err := tagexpressions.Parse(tagExpression)

	if err != nil {
		return matched, unmatched

	}

	uuid := &messages.UUID{}

	for _, filename := range files {
		f, err := sys.Open(filename)
		if err != nil {
			continue
		}
		defer f.Close()

		doc, err := gherkin.ParseGherkinDocument(f, uuid.NewId)
		if err != nil {
			log.Warn().
				Str("filename", filename).
				Msg("Could not parse file. It will be excluded from sharded execution.")
			continue
		}
		scenarios := gherkin.Pickles(*doc, filename, uuid.NewId)

		hasMatch := false
		for _, s := range scenarios {
			if match(s.Tags, tagMatcher) {
				matched = append(matched, filename)
				hasMatch = true
				break
			}
		}

		if !hasMatch {
			unmatched = append(unmatched, filename)
		}
	}
	return matched, unmatched
}

func match(tags []*messages.PickleTag, matcher tagexpressions.Evaluatable) bool {
	tagNames := make([]string, len(tags))
	for i, t := range tags {
		tagNames[i] = t.Name
	}

	return matcher.Evaluate(tagNames)
}
