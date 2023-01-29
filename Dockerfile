# Build the binary here
FROM golang:1.16 as builder
WORKDIR /go/src/github.com/saucelabs/saucectl
COPY . /go/src/github.com/saucelabs/saucectl
# TODO: Don't think this should stay
RUN go mod tidy
RUN go install cmd/saucectl/saucectl.go
# TODO: Figure out how to set the version in the Go build
RUN go build cmd/saucectl/saucectl.go

# Release the binary here
FROM ubuntu:latest  
#RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/src/github.com/saucelabs/saucectl/saucectl ./
