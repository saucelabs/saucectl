package mocks

import (
	"context"

	"github.com/saucelabs/saucectl/internal/iam"
)

type UserService struct {
	GetUserFn        func(ctx context.Context) (iam.User, error)
	GetConcurrencyFn func(ctx context.Context) (iam.Concurrency, error)
}

func (s *UserService) GetUser(ctx context.Context) (iam.User, error) {
	return s.GetUserFn(ctx)
}

func (s *UserService) GetConcurrency(ctx context.Context) (iam.Concurrency, error) {
	return s.GetConcurrencyFn(ctx)
}
