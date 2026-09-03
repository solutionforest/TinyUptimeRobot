# ---- build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /TinyUptimeRobot .

# ---- runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D monitor
WORKDIR /app
COPY --from=build /TinyUptimeRobot /app/TinyUptimeRobot
USER monitor
ENV TARGETS_FILE=/app/targets.txt \
    LOG_FILE=/app/data/status.txt \
    CHECK_INTERVAL=60s \
    HTTP_TIMEOUT=10s
VOLUME ["/app/data"]
ENTRYPOINT ["/app/TinyUptimeRobot"]
