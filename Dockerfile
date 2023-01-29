# Build the binary here
FROM golang:1.16 as builder
WORKDIR /go/src/github.com/saucelabs/saucectl
RUN go get -d -v golang.org/x/net/html  
COPY . /go/src/github.com/saucelabs/saucectl
RUN go mod download github.com/sagikazarmark/crypt
RUN go install cmd/saucectl/saucectl.go
# TODO: Figure out how to set the version
RUN go build cmd/saucectl/saucectl.go

# Release the binary here
FROM ubuntu:latest  
#RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/src/github.com/saucelabs/saucectl/saucectl ./
