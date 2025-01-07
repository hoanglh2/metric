
#Build
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
ARG GOARCH_ARG=amd64
RUN go build -o api
# Runtime
FROM alpine:3.14
WORKDIR /app
COPY --from=builder /app/api ./app-api
ENTRYPOINT ["/app/app-api"]