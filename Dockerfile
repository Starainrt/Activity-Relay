FROM golang:alpine AS build

WORKDIR /Activity-Relay
COPY . /Activity-Relay

RUN  mkdir -p /rootfs/usr/bin && \
     apk add -U --no-cache git && \
     go build -o /rootfs/usr/bin/server -ldflags "-X main.version=$(git describe --tags HEAD)" . && \
     go build -o /rootfs/usr/bin/worker -ldflags "-X main.version=$(git describe --tags HEAD)"  ./worker && \
     go build -o /rootfs/usr/bin/ar-cli -ldflags "-X main.version=$(git describe --tags HEAD)"  ./cli  && \
     go build -o /rootfs/usr/bin/spy -ldflags "-X main.version=$(git describe --tags HEAD)"  ./spy

FROM alpine

COPY --from=build /rootfs/usr/bin /usr/bin
RUN  chmod +x /usr/bin/server /usr/bin/worker /usr/bin/ar-cli /usr/bin/spy && \
     apk add -U --no-cache ca-certificates
