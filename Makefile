.PHONY: test build run clean lint coverage

# Установка зависимостей
deps:
	go mod download
	go mod tidy

# Запуск тестов
test:
	go test -v ./...

# Запуск тестов с покрытием
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Сборка проекта
build:
	go build ./...

# Запуск линтера
lint:
	# Установите golangci-lint если нет: https://golangci-lint.run/usage/install/
	golangci-lint run

# Запуск примера
run-example:
	cd examples/simple && go run main.go

# Очистка
clean:
	go clean
	rm -f coverage.out coverage.html

# Запуск Redis в Docker (для тестов)
run-redis:
	docker run -d -p 6379:6379 --name test-redis redis:7-alpine

# Остановка Redis
stop-redis:
	docker stop test-redis
	docker rm test-redis

# Помощь
help:
	@echo "Доступные команды:"
	@echo "  make deps         - Установка зависимостей"
	@echo "  make test         - Запуск тестов"
	@echo "  make test-coverage - Тесты с покрытием кода"
	@echo "  make lint         - Проверка кодстайла"
	@echo "  make run-example  - Запуск примера"
	@echo "  make run-redis    - Запуск Redis для тестов"