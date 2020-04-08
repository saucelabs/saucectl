package run

import (
	"context"
	"path/filepath"

	"github.com/saucelabs/saucectl/cli/command"
)

const logDir = "/var/log/cont"

var logFiles = [...]string{
	logDir + "/chrome_browser.log",
	logDir + "/firefox_browser.log",
	logDir + "/supervisord.log",
	logDir + "/video-rec-stderr.log",
	logDir + "/video-rec-stdout.log",
	logDir + "/wait-xvfb.1.log",
	logDir + "/wait-xvfb.2.log",
	logDir + "/wait-xvfb-stdout.log",
	logDir + "/xvfb-tryouts-stderr.log",
	logDir + "/xvfb-tryouts-stdout.log",
	"/home/seluser/videos/video.mp4",
	"/home/seluser/docker.log",
}

// ExportArtifacts exports log files from testrunner container to host
func ExportArtifacts(ctx context.Context, cli *command.SauceCtlCli, containerID string, logDir string) error {
	for _, containerSrcPath := range logFiles {
		file := filepath.Base(containerSrcPath)
		hostDstPath := filepath.Join(logDir, file)
		if err := cli.Docker.CopyFromContainer(ctx, containerID, containerSrcPath, hostDstPath); err != nil {
			cli.Logger.Info().Msgf("Couldn't find %s; dest: %s", containerSrcPath, hostDstPath)
			continue
		}
	}
	return nil
}
