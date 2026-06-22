FROM golang:1.26 AS builder

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
COPY pkg/ pkg/
COPY version/ version/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH GO111MODULE=on \
    go build -trimpath -ldflags '-w -s -buildid=' \
    -a -o plugnmeet-recorder main.go

FROM mynaparrot/plugnmeet-recorder:base

# copy files
COPY --from=builder /go/src/app/plugnmeet-recorder /usr/bin/plugnmeet-recorder
COPY docker-build/entrypoint.sh /
RUN chmod +x entrypoint.sh

ENV PATH=${PATH}:/chrome
ENTRYPOINT ["/entrypoint.sh"]
