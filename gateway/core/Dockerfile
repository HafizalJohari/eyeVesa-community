FROM rust:1.95-bookworm AS builder
RUN apt-get update && apt-get install -y protobuf-compiler && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY gateway/core/Cargo.toml gateway/core/Cargo.lock gateway/core/build.rs ./gateway/core/
COPY proto ./proto
COPY gateway/core/src ./gateway/core/src
RUN cd gateway/core && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/gateway/core/target/release/agentid-core /usr/local/bin/agentid-core
EXPOSE 9443
CMD ["agentid-core"]