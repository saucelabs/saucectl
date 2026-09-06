package authoring

import (
	"context"
	"errors"
	"fmt"

	"github.com/saucelabs/saucectl/internal/iam"
)

// ErrNotEntitled is returned when the organisation's plan does not include AI
// authoring. It is deliberately distinct from a verification failure
// (FR-032): the two need different remedies, and a user must be able to tell
// an entitlement problem from a credentials or network problem.
var ErrNotEntitled = errors.New("AI Test Authoring is not included in your Sauce Labs plan; contact your Sauce Labs account team to enable it")

// VerifyEntitlement resolves the caller's organisation and checks that it is
// entitled to AI authoring. It fails closed. Any error other than
// ErrNotEntitled means "could not verify". This costs two serial requests per
// invocation, accepted for the quality of the error (research R-009).
func VerifyEntitlement(ctx context.Context, users iam.UserService, ents EntitlementReader) (iam.User, error) {
	user, err := users.User(ctx)
	if err != nil {
		return iam.User{}, fmt.Errorf("could not verify AI authoring entitlement: resolving the current user failed: %w", err)
	}
	if user.Organization.ID == "" {
		return iam.User{}, errors.New("could not verify AI authoring entitlement: the current user has no organisation")
	}

	enabled, err := ents.IsAIAuthoringEnabled(ctx, user.Organization.ID)
	if err != nil {
		return iam.User{}, fmt.Errorf("could not verify AI authoring entitlement: %w", err)
	}
	if !enabled {
		return iam.User{}, ErrNotEntitled
	}
	return user, nil
}
