FROM golang:1.25 AS builder

ARG TARGETPLATFORM
ARG TARGETARCH
RUN echo building for "$TARGETPLATFORM"

WORKDIR /go/src/app

COPY main.go main.go
COPY go.mod go.mod
COPY go.sum go.sum
# download if above files changed
RUN go mod download

# Copy the go source
COPY helpers/ helpers/
COPY pkg/ pkg/
COPY version/ version/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH GO111MODULE=on \
    go build -trimpath -ldflags '-w -s -buildid=' \
    -a -o plugnmeet-recorder main.go

FROM ubuntu:24.04

ARG TARGETPLATFORM

# install deps
RUN apt-get update && \
    apt-get install -y curl wget gnupg unzip fonts-noto pulseaudio ffmpeg xvfb rsync

# install chrome using livekit/chrome-installer
COPY --from=livekit/chrome-installer:125.0.6422.141 /chrome-installer /chrome-installer
RUN /chrome-installer/install-chrome "$TARGETPLATFORM"

# clean up
RUN apt-get clean && rm -rf /var/lib/apt/lists/*

# copy files
COPY --from=builder /go/src/app/plugnmeet-recorder /usr/bin/plugnmeet-recorder
COPY docker-build/entrypoint.sh /
RUN chmod +x entrypoint.sh

ENV PATH=${PATH}:/chrome
ENV XDG_RUNTIME_DIR=/home/root/.cache/xdgr
ENTRYPOINT ["/entrypoint.sh"]
