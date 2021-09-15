package slack

import "context"

// Service represents an interface for retrieving slack token.
type Service interface {
	GetSlackToken(ctx context.Context) (string, error)
}
