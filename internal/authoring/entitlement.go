package authoring

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/saucelabs/saucectl/internal/iam"
)

// ErrNotEntitled is returned when the organisation's plan does not include AI
// authoring. It is deliberately distinct from a verification failure
// (FR-032): the two need different remedies, and a user must be able to tell
// an entitlement problem from a credentials or network problem.
var ErrNotEntitled = errors.New("AI Test Authoring is not included in your Sauce Labs plan; contact your Sauce Labs account team to enable it")

// VerifyEntitlement resolves the caller's organisation and checks that it is
// entitled to AI authoring. It costs two serial requests per invocation,
// accepted for the quality of the error (research R-009).
//
// The two halves fail differently, on purpose.
//
// Resolving the user is fatal when it fails: the identity it returns is not
// only used for this check. Commands such as `schedules create` send it as the
// schedule's runningUserId, so continuing with a zero-value user would write
// bad data rather than merely skip a check.
//
// The entitlement lookup is advisory. Sauce Labs enforces the entitlement on
// every authoring request, so this call only buys a clearer error sooner. When
// it cannot be completed, warn and continue: failing closed here would let an
// outage of the entitlements API block every authoring command — CI pipelines
// included — for a verdict the service delivers anyway. A definitive "not
// entitled" is still fatal, because that answer is trustworthy.
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
		log.Warn().Err(err).Msg(
			"Could not check the AI Test Authoring entitlement; continuing. Sauce Labs will reject the request if your plan does not include it.")
		return user, nil
	}
	if !enabled {
		return iam.User{}, ErrNotEntitled
	}
	return user, nil
}
