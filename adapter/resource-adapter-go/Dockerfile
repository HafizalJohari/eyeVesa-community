FROM golang:1.25-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /resource-adapter ./cmd/

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /resource-adapter /usr/local/bin/resource-adapter

EXPOSE 8443

ENTRYPOINT ["resource-adapter"]