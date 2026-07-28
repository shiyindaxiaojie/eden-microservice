FROM golang:1.25.8-alpine AS builder

WORKDIR /src

ARG TARGETARCH

COPY go.work ./
COPY packages/go.mod packages/go.mod
COPY apps/auth/go.mod apps/auth/go.mod
COPY apps/cluster/go.mod apps/cluster/go.mod
COPY apps/config/go.mod apps/config/go.mod
COPY apps/gateway/go.mod apps/gateway/go.mod
COPY apps/registry/go.mod apps/registry/go.mod
COPY apps/server/go.mod apps/server/go.mod
RUN cd apps/server && go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/eden-registry ./apps/server/cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S eden \
    && adduser -S -G eden -h /app eden \
    && mkdir -p /app/config /app/data /app/logs \
    && chown -R eden:eden /app

WORKDIR /app

COPY --from=builder /out/eden-registry /usr/local/bin/eden-registry
COPY apps/server/config/eden-microservice.yaml.example /app/config/config.yaml.example

USER eden

EXPOSE 8500 9000

ENTRYPOINT ["/usr/local/bin/eden-registry"]
CMD ["-config", "/app/config/config.yaml.example"]
