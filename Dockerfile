FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS build
WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

# Reduce rebuild time
COPY go.mod ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags "-s -w" -o /out/dnspod-updater ./cmd/dnspod-updater

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/dnspod-updater /dnspod-updater
USER nonroot:nonroot
ENTRYPOINT ["/dnspod-updater"]
