# Build the binary here
FROM golang:1.19 as builder
WORKDIR /go/src/github.com/saucelabs/saucectl
COPY . /go/src/github.com/saucelabs/saucectl
RUN go install cmd/saucectl/saucectl.go
RUN go build $LD_FLAGS cmd/saucectl/saucectl.go

# Release the binary here
FROM ubuntu:latest  
WORKDIR /root/
COPY --from=builder /go/src/github.com/saucelabs/saucectl/saucectl /usr/local/sbin/saucectl
