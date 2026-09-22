package authoring

import (
	"context"
	"errors"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/saucelabs/saucectl/internal/iam"
)

// fakeUsers and fakeEntitlements are package-local: internal/mocks imports this
// package, so an in-package test cannot use the shared fakes (Constitution IV).
type fakeUsers struct {
	userFn func(ctx context.Context) (iam.User, error)
}

func (f *fakeUsers) User(ctx context.Context) (iam.User, error) { return f.userFn(ctx) }

func (f *fakeUsers) Concurrency(context.Context) (iam.Concurrency, error) {
	return iam.Concurrency{}, nil
}

type fakeEntitlements struct {
	enabledFn func(ctx context.Context, orgID string) (bool, error)
}

func (f *fakeEntitlements) IsAIAuthoringEnabled(ctx context.Context, orgID string) (bool, error) {
	return f.enabledFn(ctx, orgID)
}

// entitledUser is a user the gate can resolve: it carries an organisation, so
// the entitlement lookup is reached.
func entitledUser() iam.User {
	return iam.User{ID: "u-1", Organization: iam.Organization{ID: "org-1"}}
}

// TestVerifyEntitlement covers the gate's two failure policies. Resolving the
// user is fatal because its identity is written to schedules; the entitlement
// lookup is advisory because the service enforces it on every request, so an
// entitlements outage must not block authoring commands.
func TestVerifyEntitlement(t *testing.T) {
	boom := errors.New("service unavailable")

	tests := []struct {
		name       string
		user       func(ctx context.Context) (iam.User, error)
		enabled    func(ctx context.Context, orgID string) (bool, error)
		wantErr    string
		wantUserID string
	}{
		{
			name:       "entitled",
			user:       func(context.Context) (iam.User, error) { return entitledUser(), nil },
			enabled:    func(context.Context, string) (bool, error) { return true, nil },
			wantUserID: "u-1",
		},
		{
			name:    "not entitled is fatal",
			user:    func(context.Context) (iam.User, error) { return entitledUser(), nil },
			enabled: func(context.Context, string) (bool, error) { return false, nil },
			wantErr: ErrNotEntitled.Error(),
		},
		{
			name:       "entitlement lookup failure is advisory",
			user:       func(context.Context) (iam.User, error) { return entitledUser(), nil },
			enabled:    func(context.Context, string) (bool, error) { return false, boom },
			wantUserID: "u-1",
		},
		{
			name:    "resolving the user is fatal",
			user:    func(context.Context) (iam.User, error) { return iam.User{}, boom },
			enabled: func(context.Context, string) (bool, error) { return true, nil },
			wantErr: "could not verify AI authoring entitlement: resolving the current user failed: service unavailable",
		},
		{
			name:    "user without an organisation is fatal",
			user:    func(context.Context) (iam.User, error) { return iam.User{ID: "u-1"}, nil },
			enabled: func(context.Context, string) (bool, error) { return true, nil },
			wantErr: "could not verify AI authoring entitlement: the current user has no organisation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUsers{userFn: tt.user}
			ents := &fakeEntitlements{enabledFn: tt.enabled}

			user, err := VerifyEntitlement(context.Background(), users, ents)

			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.wantUserID, user.ID)
		})
	}
}

// TestVerifyEntitlementAdvisoryKeepsIdentity guards the reason the lookup may
// fail open at all: the caller reuses the resolved user, so continuing must
// still hand back a usable identity rather than a zero value.
func TestVerifyEntitlementAdvisoryKeepsIdentity(t *testing.T) {
	users := &fakeUsers{
		userFn: func(context.Context) (iam.User, error) { return entitledUser(), nil },
	}
	ents := &fakeEntitlements{
		enabledFn: func(context.Context, string) (bool, error) {
			return false, errors.New("entitlements API is down")
		},
	}

	user, err := VerifyEntitlement(context.Background(), users, ents)

	assert.NilError(t, err)
	assert.Equal(t, "u-1", user.ID)
	assert.Equal(t, "org-1", user.Organization.ID)
}
