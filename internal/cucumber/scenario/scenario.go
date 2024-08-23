package scenario

import (
	"io/fs"

	gherkin "github.com/cucumber/gherkin/go/v28"
	messages "github.com/cucumber/messages/go/v24"
	"github.com/rs/zerolog/log"
)

// List parses the provided files and returns a list of scenarios.
func List(sys fs.FS, files []string) []*messages.Pickle {
	uuid := &messages.UUID{}

	var scenarios []*messages.Pickle
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
		scenarios = append(scenarios, gherkin.Pickles(*doc, filename, uuid.NewId)...)
	}
	return scenarios
}

// GetUniqueNames extracts and returns unique scenario names.
func GetUniqueNames(scenarios []*messages.Pickle) []string {
	uniqueMap := make(map[string]bool)

	var names []string
	for _, s := range scenarios {
		if uniqueMap[s.Name] {
			names = append(names, s.Name)
		}
		uniqueMap[s.Name] = true
	}
	return names
}
