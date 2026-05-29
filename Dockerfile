# Freshness-league app image.
#
# Stage 1 builds the Go binary against the module cache.
# Stage 2 ships just the binary on bare alpine.
#
# Config (PORT, PROXY, CONTESTANTS, SIDECAR_URL) is read straight from env by
# main.go, so the ENTRYPOINT can use exec form for clean signal forwarding.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY datahandling ./datahandling
COPY web ./web
COPY main.go ./
RUN go build -o /out/alterna-freshness-league .

FROM alpine:3.23
COPY --from=build /out/alterna-freshness-league /usr/local/bin/alterna-freshness-league
COPY web/content /app/web/content
WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["alterna-freshness-league"]
