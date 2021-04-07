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
	golint ./...

#format: @ Format code with gofmt
format:
	gofmt -w .

#test: @ Run tests
test:
	go test ./...

#coverage: @ Run test and check coverage
coverage:
	go test -coverprofile=coverage.out ./...
	goverreport -sort=block -order=desc -threshold=40

#playwright-ci: @ Run tests against playwright in CI mode
playwright-ci: build-linux
	docker run --name playwright-ci -e "CI=true" -v $(shell pwd):/home/gitty/ -w "/home/gitty" --rm saucelabs/stt-playwright-node:latest "/home/gitty/saucectl" run -c ./.sauce/playwright.yml --verbose

#puppeteer-ci: @ Run tests against puppeteer in CI mode
puppeteer-ci: build-linux
	docker run --name puppeteer-ci -e "CI=true" -v $(shell pwd):/home/gitty/ -w "/home/gitty" --rm saucelabs/stt-puppeteer-jest-node:latest "/home/gitty/saucectl" run -c ./.sauce/puppeteer.yml --verbose

#testcafe-ci: @ Run tests against testcafe in CI mode
testcafe-ci: build-linux
	docker run --name testcafe-ci -e "CI=true" -v $(shell pwd):/home/gitty/ -w "/home/gitty" --rm saucelabs/stt-testcafe-node:latest "/home/gitty/saucectl" run -c ./.sauce/testcafe.yml --verbose

#cypress-ci: @ Run tests against cypress in CI mode
cypress-ci: build-linux
	docker run --name cypress-ci -e "CI=true" -v $(shell pwd):/home/gitty/ -w "/home/gitty" --rm saucelabs/stt-cypress-mocha-node:latest "/home/gitty/saucectl" run -c ./.sauce/cypress.yml --verbose
