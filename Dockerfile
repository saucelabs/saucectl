# Build the binary.
FROM golang:1.23 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

ARG LD_FLAGS

COPY . .
RUN go build -ldflags="${LD_FLAGS}" cmd/saucectl/saucectl.go

# Bundle the binary.
FROM ubuntu:latest

RUN apt-get update && apt-get install -y ca-certificates && update-ca-certificates

COPY --from=builder /app/saucectl /usr/local/bin/saucectl
