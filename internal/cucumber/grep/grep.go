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
	for _, f := range files {
		g, err := sys.Open(f)
		if err != nil {
			continue
		}
		defer g.Close()

		uuid := &messages.UUID{}
		doc, err := gherkin.ParseGherkinDocument(g, uuid.NewId)
		if err != nil {
			continue
		}
		pickles := gherkin.Pickles(*doc, f, uuid.NewId)

		hasMatch := false
		for _, p := range pickles {
			if match(p.Tags, tagMatcher) {
				matched = append(matched, f)
				hasMatch = true
				break
			}
		}

		if !hasMatch {
			unmatched = append(unmatched, f)
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
