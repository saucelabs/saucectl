# Build the binary here
FROM golang:1.19 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

ARG LD_FLAGS

COPY . .
RUN go build -ldflags="${LD_FLAGS}" cmd/saucectl/saucectl.go

# Release the binary here
FROM ubuntu:latest
COPY --from=builder /app/saucectl /usr/local/bin/saucectl
