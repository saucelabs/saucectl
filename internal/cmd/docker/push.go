package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
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
	var quiet bool

	cmd := &cobra.Command{
		Use:          "push <IMAGE_NAME>",
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
			repo, err := extractRepo(image)
			fmt.Println("repo: ", repo)
			if err != nil {
				return err
			}
			auth, err := imageRunnerService.RegistryLogin(context.Background(), repo)
			if err != nil {
				return fmt.Errorf("failed to fetch auth token: %v", err)
			}
			return pushDockerImage(image, auth.Username, auth.Password, registryPushTimeout, quiet)
		},
	}

	flags := cmd.PersistentFlags()
	flags.DurationVar(&registryPushTimeout, "registry-push-timeout", 5*time.Minute, "Configure the timeout duration for docker push . Default: 5 minute.")
	flags.BoolVar(&quiet, "quiet", false, "Run silently, suppressing output messages.")

	return cmd
}

func pushDockerImage(imageName, username, password string, registryPushTimeout time.Duration, quiet bool) error {
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

	if quiet {
		return nil
	}

	// Print the push output
	_, err = io.Copy(os.Stdout, out)
	if err != nil {
		return fmt.Errorf("docker output: %v", err)
	}

	return nil
}

func extractRepo(input string) (string, error) {
	// Example: us-west4-docker.pkg.dev/sauce-hto-p-jy6b/repo-name/sub-folder/../ubuntu:experiment
	regexPattern := `^us-west4-docker\.pkg\.dev/sauce-hto-p-jy6b/([^/]+)/.*$`

	re := regexp.MustCompile(regexPattern)
	if !re.MatchString(input) {
		return "", fmt.Errorf("invalid docker image name")
	}

	matches := re.FindStringSubmatch(input)

	if len(matches) >= 2 {
		return matches[1], nil
	}

	return "", fmt.Errorf("unable to extract repo name from the image")
}
