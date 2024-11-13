package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	dockermsg "github.com/moby/moby/pkg/jsonmessage"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/progress"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func PushCommand() *cobra.Command {
	var timeout time.Duration
	var quiet bool

	cmd := &cobra.Command{
		Use:          "push <image_name>",
		Short:        "Push a Docker image to the Sauce Labs Container Registry.",
		SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no docker image specified")
			}

			return nil
		},
		PreRun: func(cmd *cobra.Command, _ []string) {
			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Properties{}.SetFlags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
		},
		RunE: func(_ *cobra.Command, args []string) error {
			image := args[0]
			auth, err := imageRunnerService.RegistryLogin(context.Background(), image)
			if err != nil {
				return fmt.Errorf("failed to fetch auth token: %v", err)
			}
			return pushDockerImage(image, auth.Username, auth.Password, timeout, quiet)
		},
	}

	flags := cmd.PersistentFlags()
	flags.DurationVar(&timeout, "timeout", 5*time.Minute, "Configure the timeout duration for docker push.")
	flags.BoolVar(&quiet, "quiet", false, "Run silently, suppressing output messages.")

	return cmd
}

func pushDockerImage(imageName, username, password string, timeout time.Duration, quiet bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %v", err)
	}

	authConfig := registry.AuthConfig{
		Username: username,
		Password: password,
	}

	authBytes, err := json.Marshal(authConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal docker auth: %v", err)
	}
	authBase64 := base64.URLEncoding.EncodeToString(authBytes)

	// Push the image to the registry
	pushOptions := types.ImagePushOptions{RegistryAuth: authBase64}
	out, err := cli.ImagePush(ctx, imageName, pushOptions)
	if err != nil {
		return fmt.Errorf("failed to push image: %v", err)
	}
	defer out.Close()

	if quiet {
		return nil
	}

	return logPushProgress(out)
}

func logPushProgress(reader io.ReadCloser) error {
	var stepID string
	bars := map[string]*progressbar.ProgressBar{}
	decoder := json.NewDecoder(reader)
	for {
		// Decode the message from the Docker API output.
		var msg dockermsg.JSONMessage
		err := decoder.Decode(&msg)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode JSON during Docker image push: %v", err)
		}
		if msg.Error != nil {
			return fmt.Errorf("server error during Docker image push: %s", msg.Error.Message)
		}

		// Create a new progress spinner or bar when the Docker push ID changes.
		// Each ID corresponds to a distinct step in the push process.
		// Note: Outputs are in parallel; identical IDs indicate outputs from the same thread.
		if stepID != msg.ID {
			stepID = msg.ID
			progress.Stop()

			// Create a progress spinner for statuses that don't have progress details, like 'Preparing'.
			if msg.Progress == nil || msg.Progress.Total == 0 {
				progress.Show(msg.Status, nil)
				continue
			}

			// Init a progress bar for 'Pushing' status with total bytes and add it to bar group.
			if _, ok := bars[msg.ID]; !ok {
				bars[msg.ID] = createBar(msg.Progress.Total, fmt.Sprintf("%s %s", msg.Status, msg.ID))
			}
		}

		// Update current progress based on msg.Progress.Total when in 'Pushing' status.
		// Note: The Docker API may return a current value greater than total. To prevent breaking the progress bar,
		// only update when the current is less than total.
		if bar, ok := bars[msg.ID]; ok {
			if msg.Progress != nil && msg.Progress.Current > 0 && msg.Progress.Current < bar.GetMax64() {
				_ = bar.Set64(msg.Progress.Current)
			}
		}
	}
	if err := closeProgress(bars); err != nil {
		return err
	}

	fmt.Println("Successfully pushed the Docker image!")
	return nil
}

// createBar returns a customized progress bar for Docker image pushes.
func createBar(max int64, desc string) *progressbar.ProgressBar { //nolint:revive
	return progressbar.NewOptions64(
		max,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        ">]",
		}),
	)
}

// closeProgress closes all progress spinners and bars.
func closeProgress(bars map[string]*progressbar.ProgressBar) error {
	progress.Stop()
	for _, bar := range bars {
		if err := bar.Finish(); err != nil {
			return err
		}
	}
	return nil
}
