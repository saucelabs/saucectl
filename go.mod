module github.com/saucelabs/saucectl

go 1.14

// Docker's last compatible version with x/sys/windows
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/hcsshim v0.8.9 // indirect
	github.com/briandowns/spinner v1.11.1
	github.com/containerd/continuity v0.0.0-20200710164510-efbc4488d8fe // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible // translates to v19.03.12
	github.com/docker/go-connections v0.4.0
	github.com/google/go-github/v32 v32.1.0
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/jarcoal/httpmock v1.0.6
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.8.1
	github.com/rs/zerolog v1.18.0
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.4.0
	github.com/tj/survey v2.0.6+incompatible
	gopkg.in/yaml.v2 v2.2.8
	gotest.tools v2.2.0+incompatible
	gotest.tools/v3 v3.0.2
)
