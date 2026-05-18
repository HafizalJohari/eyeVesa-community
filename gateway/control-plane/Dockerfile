FROM golang:1.25-bookworm AS builder
WORKDIR /app
COPY registry/migrations /app/registry/migrations
COPY gateway/control-plane /app/gateway/control-plane
COPY proto /app/proto
RUN cd gateway/control-plane && go mod download
RUN cd gateway/control-plane && CGO_ENABLED=0 go build -o /agentid-control ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /agentid-control /usr/local/bin/agentid-control
COPY --from=builder /app/gateway/control-plane/policies /policies
COPY --from=builder /app/registry/migrations /registry/migrations
ENV MIGRATIONS_DIR=/registry/migrations
EXPOSE 8080 9090
CMD ["agentid-control"]