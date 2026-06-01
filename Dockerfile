# Multi-stage build shared by all webx services.
#
# Build a specific service by passing the SERVICE build arg, e.g.
#   docker build --build-arg SERVICE=orders -t webx-orders .
# docker-compose.yml does this automatically for each service.

# ── builder ──────────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# goproxy.cn keeps module downloads reliable; "direct" is the fallback.
ENV GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build the requested service.
COPY . .
ARG SERVICE
RUN test -n "$SERVICE" || (echo "SERVICE build arg is required" && exit 1)
RUN go build -trimpath -ldflags="-s -w" -o /out/app ./internal/${SERVICE}

# ── runtime ──────────────────────────────────────────────────────────────────
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /out/app /app/app
# Migration files (resources/<service>/*.sql) and the merged OpenAPI spec used
# by the gateway's /docs endpoint.
COPY --from=builder /src/resources /app/resources
COPY --from=builder /src/proto_go/webx.swagger.json /app/proto_go/webx.swagger.json

ENTRYPOINT ["/app/app"]
