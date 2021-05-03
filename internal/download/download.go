package download

import "github.com/saucelabs/saucectl/internal/config"

type Downloader interface {
	DownloadArtifacts(config.ArtifactDownload, string, bool)
}
