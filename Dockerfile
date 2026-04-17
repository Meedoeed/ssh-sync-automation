FROM golang:1.25.2-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ssh-sync-service ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/ssh-sync-service .

COPY --from=builder /app/internal/storage/postgres/migrations ./internal/storage/postgres/migrations

RUN mkdir -p /app/data/done /app/data/tasks

EXPOSE 8081

CMD ["./ssh-sync-service"]