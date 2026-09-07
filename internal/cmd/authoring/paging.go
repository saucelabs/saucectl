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
	// limitChanged records whether the user actually set --limit, which is
	// the only way to tell an explicit value from the default when deciding
	// whether to warn that --all cannot honour it.
	limitChanged bool
	// jsonOut suppresses advisory warnings. The logger writes to stdout for
	// every saucectl command, so a warning emitted while rendering JSON
	// lands in the middle of the document and breaks `| jq`. Silence is
	// wrong in general (Constitution VIII), which is why this only applies
	// to advice the user cannot act on mid-listing.
	jsonOut bool
}

// bind registers the flags.
func (p *pageFlags) bind(fs *pflag.FlagSet) {
	fs.IntVar(&p.skip, "skip", 0, "Number of results to skip.")
	fs.IntVar(&p.limit, "limit", 20, "Maximum number of results to return. 0 returns only the total count.")
	fs.BoolVar(&p.all, "all", false, "Return every result, fetching all pages.")
}

// capture records flag state that cannot be read from the values alone.
// Call it from RunE, before fetching.
func (p *pageFlags) capture(fs *pflag.FlagSet) {
	p.limitChanged = fs.Changed("limit")
	out, err := fs.GetString("out")
	p.jsonOut = err == nil && out == JSONOutput
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

	// --all fetches whole pages at a fixed size, so --limit cannot be
	// honoured. Say so rather than ignoring it silently (Constitution VIII).
	if p.limitChanged && !p.jsonOut {
		log.Warn().Msgf("--limit is ignored with --all; every %s is fetched in pages of %d.", resource, authoring.DefaultPageSize)
	}

	warned := false
	items, err := authoring.ListAll(ctx, authoring.DefaultPageSize, func(ctx context.Context, opts authoring.ListOptions) (authoring.List[T], error) {
		// --skip still means "start here": offset every page by it, so
		// `--all --skip 100` does not silently restart from the beginning.
		opts.Skip += p.skip
		l, err := fetch(ctx, opts)
		if err == nil && !warned && !p.jsonOut && l.Total > largeListingThreshold {
			warned = true
			log.Warn().Msgf("Fetching all %d %s; large listings take a while and transfer several megabytes.", l.Total, resource)
		}
		return l, err
	})
	return items, len(items), err
}

// jobURL derives a job's dashboard link in the region resolved for this
// invocation. The derivation itself lives in internal/authoring so the run
// results table and these tables cannot drift apart.
func jobURL(sauceJobID string) string {
	return authoring.JobURL(regio, sauceJobID)
}
