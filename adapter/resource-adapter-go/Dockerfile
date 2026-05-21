FROM golang:1.26-bookworm AS builder
WORKDIR /app
COPY adapter/resource-adapter-go/go.mod adapter/resource-adapter-go/go.sum ./adapter/resource-adapter-go/
COPY adapter/resource-adapter-go ./adapter/resource-adapter-go
RUN cd adapter/resource-adapter-go && go mod download && CGO_ENABLED=0 GOOS=linux go build -o /resource-adapter ./cmd/

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /resource-adapter /usr/local/bin/resource-adapter
EXPOSE 8443
ENTRYPOINT ["resource-adapter"]