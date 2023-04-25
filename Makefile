default:
	@grep -E '[a-zA-Z]+:.*?@ .*$$' $(MAKEFILE_LIST)| tr -d '#' | awk 'BEGIN {FS = ":.*?@ "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

#install: @ Install the CLI
.PHONY: install
install:
	GOBIN=/usr/local/bin/ go install cmd/saucectl/saucectl.go

#build: @ Build the CLI
build:
	go build cmd/saucectl/saucectl.go

build-%:
	GOOS=$* GOARCH=amd64 make build

#lint: @ Run the linter
lint:
	golangci-lint run

#format: @ Format code with gofmt
format:
	gofmt -w .

#test: @ Run tests
test:
	go test ./...

#coverage: @ Run test and check coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@rm coverage.out

#testcafe-ci: @ Run tests against testcafe in CI mode
testcafe-ci: build-linux
	docker run --name testcafe-ci -e "CI=true" -v $(shell pwd):/home/gitty/ -w "/home/gitty" --rm saucelabs/stt-testcafe-node:latest "/home/gitty/saucectl" run -c ./.sauce/testcafe.yml --verbose

#schema: @ Build the json schema
schema:
	$(eval INPUT_SCHEMA := $(shell pwd)/api/global.schema.json)
	$(eval OUTPUT_SCHEMA := $(shell pwd)/api/saucectl.schema.json)
	pushd scripts/json-schema-bundler/ && \
	npm run bundle -- -s $(INPUT_SCHEMA) -o $(OUTPUT_SCHEMA) && \
	popd
