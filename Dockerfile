FROM golang:1.6
MAINTAINER Octoblu, Inc. <docker@octoblu.com>

WORKDIR /go/src/github.com/octoblu/governator
COPY . /go/src/github.com/octoblu/governator

RUN env CGO_ENABLED=0 go build -a -ldflags '-s' .

CMD ["./governator"]
