package mocks

import (
	"context"

	"github.com/saucelabs/saucectl/internal/iam"
)

type UserService struct {
	UserFn        func(ctx context.Context) (iam.User, error)
	ConcurrencyFn func(ctx context.Context) (iam.Concurrency, error)
}

func (s *UserService) User(ctx context.Context) (iam.User, error) {
	return s.UserFn(ctx)
}

func (s *UserService) Concurrency(ctx context.Context) (iam.Concurrency, error) {
	return s.ConcurrencyFn(ctx)
}
