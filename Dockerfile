FROM golang:1.23-alpine AS builder
WORKDIR /app

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=

COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -trimpath -ldflags="-s -w" -o /music-bot ./cmd/bot

FROM alpine:3.21
WORKDIR /app
RUN adduser -D app && mkdir -p /app/data/downloads && chown -R app:app /app
COPY --from=builder /music-bot /usr/local/bin/music-bot
USER app
ENV DOWNLOAD_DIR=/app/data/downloads
ENV CONFIG_FILE=/app/config/config.yaml
ENV BOT_TOKEN_FILE=/run/secrets/telegram-bot-token
ENTRYPOINT ["/usr/local/bin/music-bot"]
