package authoring

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// DownloadArtifactCommand is `authoring download-artifact`: it fetches a file
// captured during authoring, such as a step screenshot, by its identifier.
func DownloadArtifactCommand() *cobra.Command {
	var filename string
	var force bool

	cmd := &cobra.Command{
		Use:     "download-artifact <artifact-id> -f <path>",
		Aliases: []string{"artifact"},
		Short:   "Download an artifact captured during authoring, such as a step screenshot",
		Long: `Download an artifact by its identifier.

The identifier is the last path segment of a step's screenshot URL, as shown by
'testcases get --show-steps'. A full screenshot URL is also accepted. The download
carries no content type, so the destination filename is required.`,
		Example:      `  saucectl authoring download-artifact 3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b -f step-3.png`,
		SilenceUsage: true,
		Args:         requireArgs("artifact-id"),
		PreRun: func(cmd *cobra.Command, _ []string) {
			trackUsage(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if filename == "" {
				return errors.New("a destination is required: use -f/--filename")
			}
			id := authoring.ArtifactIDFromURL(args[0])
			if id == "" {
				return fmt.Errorf("invalid artifact identifier %q", args[0])
			}
			return downloadArtifact(cmd, id, filename, force)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&filename, "filename", "f", "", "Destination file. Required, because the response carries no content type from which an extension could be inferred.")
	flags.BoolVar(&force, "force", false, "Overwrite the destination if it exists.")

	return cmd
}

// downloadArtifact streams the artifact to filename, refusing to overwrite
// unless forced (FR-031 applies to every file this tool writes).
func downloadArtifact(cmd *cobra.Command, id, filename string, force bool) error {
	if !force {
		if _, err := os.Stat(filename); err == nil {
			return fmt.Errorf("%s already exists; use --force to overwrite", filename)
		}
	}

	rc, err := artifactService.DownloadArtifact(cmd.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to download artifact: %w", err)
	}
	defer rc.Close()

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", filename, err)
	}
	defer f.Close()

	n, err := io.Copy(f, rc)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}
	fmt.Printf("Wrote %d bytes to %s\n", n, filename)
	return nil
}
