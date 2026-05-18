FROM golang:1.26-bookworm AS builder
WORKDIR /app
COPY gateway/control-plane/go.mod gateway/control-plane/go.sum ./gateway/control-plane/
COPY proto ./proto
COPY registry/migrations ./registry/migrations
COPY gateway/control-plane ./gateway/control-plane
RUN cd gateway/control-plane && go mod download && CGO_ENABLED=0 go build -o /agentid-control ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /agentid-control /usr/local/bin/agentid-control
COPY --from=builder /app/gateway/control-plane/policies /policies
COPY --from=builder /app/registry/migrations /registry/migrations
ENV MIGRATIONS_DIR=/registry/migrations
EXPOSE 8080 9090
CMD ["agentid-control"]