package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	dockerMsg "github.com/moby/moby/pkg/jsonmessage"
	cmds "github.com/saucelabs/saucectl/internal/cmd"
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
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no docker image specified")
			}

			return nil
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			tracker := segment.DefaultTracker

			go func() {
				tracker.Collect(
					cases.Title(language.English).String(cmds.FullName(cmd)),
					usage.Properties{}.SetFlags(cmd.Flags()),
				)
				_ = tracker.Close()
			}()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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
	var status string
	var msg dockerMsg.JSONMessage
	var bar *progressbar.ProgressBar

	decoder := json.NewDecoder(reader)
	for {
		// Decode the message from the Docker API output.
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

		// Create a new progress bar to display progress whenever the Docker push status changes.
		if status != msg.Status {
			status = msg.Status
			// Create a spinner-based progress bar for statuses other than 'pushing', like 'prepare'.
			if msg.Progress == nil || msg.Progress.Total == 0 {
				bar = progressbar.Default(-1, status)
				continue
			}
			// Create a new progress bar for 'pushing' status with total bytes.
			bar = progressbar.Default(msg.Progress.Total, status)
		}

		// Update current progress based on msg.Progress.Total when in 'pushing' status.
		if bar != nil && msg.Progress != nil && msg.Progress.Current > 0 {
			bar.Set64(msg.Progress.Current)
		}
	}

	if bar != nil {
		bar.Finish()
	}
	fmt.Println("\nSuccessfully pushed the Docker image!")
	return nil
}
