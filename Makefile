default:
	@grep -E '[a-zA-Z]+:.*?@ .*$$' $(MAKEFILE_LIST)| tr -d '#' | awk 'BEGIN {FS = ":.*?@ "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

#install: @ Install all depencies defined in package.json
install:
	brew install golangci/tap/golangci-lint
	go get ./...

#build: @ Build the CLI
build:
	go build cmd/saucectl/saucectl.go

#lint: @ Run the linter
lint:
	golint ./...

#format: @ Format code with gofmt
format:
	gofmt -w .

#test: @ Run tests
test:
	go test -v ./...
