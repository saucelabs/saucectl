module github.com/saucelabs/saucectl

go 1.16

// Docker's last compatible version with x/sys/windows
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a

require (
	github.com/AlecAivazis/survey/v2 v2.2.12
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/hcsshim v0.8.9 // indirect
	github.com/Netflix/go-expect v0.0.0-20180615182759-c93bf25de8e8
	github.com/bmatcuk/doublestar/v4 v4.0.2
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/briandowns/spinner v1.11.1
	github.com/containerd/continuity v0.0.0-20200710164510-efbc4488d8fe // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible // translates to v19.03.12
	github.com/docker/go-connections v0.4.0
	github.com/fatih/color v1.9.0
	github.com/getsentry/sentry-go v0.10.0
	github.com/go-git/go-git/v5 v5.2.0
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/hinshun/vt10x v0.0.0-20180616224451-1954e6464174
	github.com/jarcoal/httpmock v1.0.6
	github.com/jedib0t/go-pretty/v6 v6.2.1
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rs/zerolog v1.18.0
	github.com/ryanuber/go-glob v1.0.0
	github.com/segmentio/backo-go v0.0.0-20200129164019-23eae7c10bd3 // indirect
	github.com/slack-go/slack v0.9.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.9.0
	github.com/stretchr/testify v1.7.0
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	golang.org/x/mod v0.4.2
	gopkg.in/segmentio/analytics-go.v3 v3.1.0
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools v2.2.0+incompatible
	gotest.tools/v3 v3.0.2
)
