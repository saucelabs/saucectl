package setup

// Default build-time variable to setup Sentry.
// These values are overridden via ldflags
var (
	SentryDSN = "unknown-sentry-dsn"
)
