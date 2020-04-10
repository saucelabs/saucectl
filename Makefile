default:
	@echo "saucectl CLI"
	# ToDo(Christian): add some output for documentation purposes

install:
	go get ./...

build:
	go build cmd/saucectl/saucectl.go

test:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out