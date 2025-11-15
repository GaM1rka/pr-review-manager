# Имя бинарника
BINARY_NAME=app

# Запуск сборки
build:
	go build -o bin/$(BINARY_NAME) ./cmd/app

# Запуск локально
run: build
	./bin/$(BINARY_NAME)

# Запуск в docker-compose
up:
	docker-compose up --build

# Остановка и удаление контейнеров
down:
	docker-compose down

# Запуск линтера
lint:
	golangci-lint run

# Очистка артефактов сборки
clean:
	rm -rf bin/
