# Шаг 1: Сборка приложения
FROM golang:1.21 AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы проекта
COPY . .

# Загружаем зависимости и собираем бинарник
RUN go mod tidy && go build -o pipeline main.go

# Шаг 2: Создание минимального образа для запуска
FROM alpine:latest

WORKDIR /root/

# Копируем бинарный файл из builder-этапа
COPY --from=builder /app/pipeline .

# Запускаем приложение
CMD ["./pipeline"]