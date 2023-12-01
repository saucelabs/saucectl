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
	cmds "github.com/saucelabs/saucectl/internal/cmd"
	"github.com/saucelabs/saucectl/internal/segment"
	"github.com/saucelabs/saucectl/internal/usage"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func PushCommand() *cobra.Command {
	var registryPushTimeout time.Duration

	cmd := &cobra.Command{
		Use:          "push <REPO> <IMAGE_NAME>",
		Short:        "Push a Docker image to the Sauce Labs Container Registry.",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || args[0] == "" {
				return errors.New("no Sauce Labs repo specified")
			}
			if len(args) == 1 || args[1] == "" {
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
			repo := args[0]
			image := args[1]
			auth, err := registryClient.Login(context.Background(), repo)
			if err != nil {
				return fmt.Errorf("failed to fetch auth token: %v", err)
			}
			return pushDockerImage(image, auth.Username, auth.Password, registryPushTimeout)
		},
	}

	flags := cmd.PersistentFlags()
	flags.DurationVar(&registryPushTimeout, "registry-push-timeout", 1*time.Minute, "Set timeout for docker push. Default: 1 minute.")

	return cmd
}

func pushDockerImage(imageName, username, password string, registryPushTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), registryPushTimeout)
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

	// Print the push output
	_, err = io.Copy(os.Stdout, out)
	if err != nil {
		return fmt.Errorf("docker output: %v", err)
	}

	return nil
}
