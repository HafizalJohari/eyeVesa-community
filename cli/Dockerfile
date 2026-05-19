FROM golang:1.25-alpine AS build
WORKDIR /src

COPY proto /src/proto
COPY cli /src/cli

WORKDIR /src/cli
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/eyevesa .

FROM alpine:3.20
RUN adduser -D -H eyevesa
COPY --from=build /out/eyevesa /usr/local/bin/eyevesa
USER eyevesa
ENTRYPOINT ["eyevesa"]
