package msg

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"strings"
)

// LogTestSuccess prints out a test success summary statement.
func LogTestSuccess() {
	log.Info().Msg("┌───────────────────────┐")
	log.Info().Msg(" All suites have passed! ")
	log.Info().Msg("└───────────────────────┘")
}

// LogTestFailure prints out a test failure summary statement.
func LogTestFailure(errors, total int) {
	relative := float64(errors) / float64(total) * 100
	msg := fmt.Sprintf(" %d of %d suites have failed (%.0f%%) ", errors, total, relative)
	dashes := strings.Repeat("─", len(msg)-2)
	log.Error().Msgf("┌%s┐", dashes)
	log.Error().Msg(msg)
	log.Error().Msgf("└%s┘", dashes)
}
