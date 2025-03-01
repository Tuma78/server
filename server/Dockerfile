# Используем официальный образ Go
FROM golang:1.24 AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы и устанавливаем зависимости
COPY go.mod ./
RUN go mod download

# Копируем исходники
COPY . .

# Сборка бинарника
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd  # Убедитесь, что путь верный

# Минимальный образ для запуска
FROM alpine:latest

WORKDIR /root/

# Копируем скомпилированный бинарник из builder-а
COPY --from=builder /app/server .

# Делаем бинарник исполнимым (если нужно)
RUN chmod +x /root/server

# Запускаем приложение
CMD ["./server"]
