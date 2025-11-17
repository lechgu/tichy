FROM golang:alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o tichy .

FROM alpine:latest

RUN apk --no-cache add curl

COPY --from=builder /build/tichy /local/bin/tichy

ENTRYPOINT ["/local/bin/tichy"]
