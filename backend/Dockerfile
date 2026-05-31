# syntax=docker/dockerfile:1
FROM --platform=linux/amd64 golang:alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /app/server ./cmd/server

FROM --platform=linux/amd64 alpine:latest AS runtime
RUN adduser -D appuser
WORKDIR /app
COPY --from=builder /app/server .
USER appuser
EXPOSE 8080
CMD ["./server"]
