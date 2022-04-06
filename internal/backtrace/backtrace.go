package backtrace

import (
	"os"

	bt "github.com/backtrace-labs/backtrace-go"
	"github.com/rs/zerolog/log"
)

func Report(err error, opts map[string]interface{}, cfgFile string) {
	if cfgFile != "" {
		opts["configFile"] = readConfigFile(cfgFile)
	}
	bt.Report(err, opts)
	bt.FinishSendingReports()
}

func readConfigFile(file string) string {
	cfg, err := os.ReadFile(file)
	if err != nil {
		log.Err(err).Msg("Failed to upload config file to backtrace")
		return ""
	}

	return string(cfg)
}
