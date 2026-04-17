FROM golang:1.25.2-alpine AS builder

WORKDIR /app

# Устанавливаем зависимости
RUN apk add --no-cache git ca-certificates

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ssh-sync-service ./cmd/server

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Копируем бинарник
COPY --from=builder /app/ssh-sync-service .

# Копируем миграции
COPY --from=builder /app/internal/storage/postgres/migrations ./internal/storage/postgres/migrations

# Создаем директории для данных
RUN mkdir -p /app/data/done /app/data/tasks

EXPOSE 8081

CMD ["./ssh-sync-service"]