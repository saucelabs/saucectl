package concurrency

import (
	"context"

	"github.com/rs/zerolog/log"
)

// Min reads the allowed concurrency from r, compares it against ccy and returns the smaller value of the two.
// A value of 1 is returned if r is unable to provide one.
func Min(r Reader, ccy int) int {
	allowed, err := r.ReadAllowedCCY(context.Background())
	if err != nil {
		log.Warn().Err(err).Msg("Unable to determine allowed concurrency. Using concurrency of 1.")
		return 1
	}

	if ccy > allowed {
		log.Warn().Msgf("Allowed concurrency is %d. Overriding configured value of %d.",
			allowed, ccy)
		return allowed
	}

	return ccy
}

// SplitTestFiles splits test files into groups to match concurrency
func SplitTestFiles(files []string, concurrency int) [][]string {
	if concurrency == 1 {
		return [][]string{files}
	}
	groups := [][]string{}
	fileCount := len(files)
	if concurrency > fileCount {
		concurrency = fileCount // if concurrency amount is bigger than fileCount, then run one testfile per concurrency
	}
	groupLen := fileCount / concurrency
	var count, i int
	for count < concurrency {
		group := []string{}
		for j := i; j < fileCount && j < i+groupLen; j++ {
			group = append(group, files[j])
		}
		groups = append(groups, group)
		i += groupLen
		count++
	}
	// if filecount cannot be devided by concurrency, then there are some files left. Merge left files into the last group
	for i < fileCount {
		groups[concurrency-1] = append(groups[concurrency-1], files[i])
		i++
	}

	return groups
}
