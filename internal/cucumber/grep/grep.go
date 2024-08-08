package grep

import (
	"github.com/cucumber/tag-expressions/go/v6"
	"io/fs"

	gherkin "github.com/cucumber/gherkin/go/v28"
	messages "github.com/cucumber/messages/go/v24"
)

func MatchFiles(sys fs.FS, files []string, tag string) (matched []string, unmatched []string) {
	tagMatcher, err := tagexpressions.Parse(tag)

	if err != nil {
		return matched, unmatched

	}
	for _, filename := range files {
		f, err := sys.Open(filename)
		if err != nil {
			continue
		}
		defer f.Close()

		uuid := &messages.UUID{}
		doc, err := gherkin.ParseGherkinDocument(f, uuid.NewId)
		if err != nil {
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
