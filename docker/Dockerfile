FROM golang:1.25.8-alpine AS builder

WORKDIR /src

ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/eden-registry ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S eden \
    && adduser -S -G eden -h /app eden \
    && mkdir -p /app/config /app/data /app/logs \
    && chown -R eden:eden /app

WORKDIR /app

COPY --from=builder /out/eden-registry /usr/local/bin/eden-registry
COPY config/config.yaml.example /app/config/config.yaml.example

USER eden

EXPOSE 8500 9000

ENTRYPOINT ["/usr/local/bin/eden-registry"]
CMD ["-config", "/app/config/config.yaml.example"]
