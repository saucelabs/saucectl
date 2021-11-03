package logger

import "github.com/rs/zerolog/log"

type Logger struct{}

func (*Logger) Printf(format string, v ...interface{}) {
	log.Debug().Msgf(format, v...)
}
