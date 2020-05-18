default:
	@echo "saucectl CLI"
	# ToDo(Christian): add some output for documentation purposes

install:
	brew install golangci/tap/golangci-lint
	go get ./...

build:
	go build cmd/saucectl/saucectl.go

lint:
	golangci-lint run ./... --disable structcheck

format:
	gofmt -w .

test:
	go test -coverprofile=coverage.out ./...
	goverreport -sort=block -order=desc -threshold=44