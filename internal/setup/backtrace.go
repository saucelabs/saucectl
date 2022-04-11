package setup

// Default build-time variable to setup Backtrace.
// These values are overridden via ldflags
var (
	BackTraceEndpoint = "unknown-backtrace-endpoint"
	BackTraceToken    = "unknown-backtrace-token"
)
