package authoring

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// largeListingThreshold is the total above which --all warns before fetching
// everything: a test case listing weighs ~13 KB per case.
const largeListingThreshold = 200

// pageFlags are the pagination flags every listing command shares. They mirror
// the service's skip/limit rather than `builds`' page/size because users of
// the service's own API already think in those terms.
type pageFlags struct {
	skip  int
	limit int
	all   bool
}

// bind registers the flags.
func (p *pageFlags) bind(fs *pflag.FlagSet) {
	fs.IntVar(&p.skip, "skip", 0, "Number of results to skip.")
	fs.IntVar(&p.limit, "limit", 20, "Maximum number of results to return. 0 returns only the total count.")
	fs.BoolVar(&p.all, "all", false, "Return every result, fetching all pages.")
}

// validate rejects negative values before any request.
func (p pageFlags) validate() error {
	if p.skip < 0 {
		return fmt.Errorf("--skip must not be negative")
	}
	if p.limit < 0 {
		return fmt.Errorf("--limit must not be negative")
	}
	return nil
}

// options converts the flags into list options. The limit is always sent
// because the flag always has a value; 0 is the count-only request.
func (p pageFlags) options() authoring.ListOptions {
	limit := p.limit
	return authoring.ListOptions{Skip: p.skip, Limit: &limit}
}

// fetchPage fetches one page, or every page under --all, through fetch. It
// returns the items and the total the service reports, so the footer can say
// how many results exist beyond those shown (FR-035).
func fetchPage[T any](ctx context.Context, p pageFlags, resource string, fetch func(context.Context, authoring.ListOptions) (authoring.List[T], error)) ([]T, int, error) {
	if !p.all {
		l, err := fetch(ctx, p.options())
		return l.Items, l.Total, err
	}

	warned := false
	items, err := authoring.ListAll(ctx, authoring.DefaultPageSize, func(ctx context.Context, opts authoring.ListOptions) (authoring.List[T], error) {
		l, err := fetch(ctx, opts)
		if err == nil && !warned && l.Total > largeListingThreshold {
			warned = true
			log.Warn().Msgf("Fetching all %d %s; large listings take a while and transfer several megabytes.", l.Total, resource)
		}
		return l, err
	})
	return items, len(items), err
}

// jobURL derives a job's dashboard link from its Sauce job identifier. The
// service's own url field is present on only about half of jobs (research
// R-006), so it is never relied upon.
func jobURL(sauceJobID string) string {
	if sauceJobID == "" {
		return ""
	}
	return regio.AppBaseURL() + "/tests/" + sauceJobID
}
