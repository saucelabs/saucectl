default:
	@echo "saucectl CLI"
	# ToDo(Christian): add some output for documentation purposes

install:
	go get ./...

build:
	go build

test:
	go test ./...