FROM golang:1.16 as builder

ARG GIT_TOKEN

RUN go env -w GOPRIVATE=github.com/allinbits/*
RUN git config --global url."https://git:${GIT_TOKEN}@github.com".insteadOf "https://github.com"

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOPROXY=direct make rpcwatcher

FROM alpine:latest

RUN apk --no-cache add ca-certificates mailcap && addgroup -S app && adduser -S app -G app
COPY --from=builder /app/build/rpcwatcher /usr/local/bin/rpcwatcher
USER app
ENTRYPOINT ["/usr/local/bin/rpcwatcher"]