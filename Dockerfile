FROM golang:1.7
MAINTAINER Octoblu, Inc. <docker@octoblu.com>

WORKDIR /go/src/github.com/octoblu/governator-swarm
COPY . /go/src/github.com/octoblu/governator-swarm

RUN env CGO_ENABLED=0 go build -a -ldflags '-s' .

CMD ["./governator-swarm"]
