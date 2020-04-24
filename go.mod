module github.com/saucelabs/saucectl

go 1.14

replace github.com/Sirupsen/logrus v1.5.0 => github.com/sirupsen/logrus v1.5.0

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/Sirupsen/logrus v1.5.0 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.4.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/rs/zerolog v1.18.0
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.2.2
	gopkg.in/yaml.v2 v2.2.8
	gotest.tools/v3 v3.0.2
)
