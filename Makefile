.PHONY: test test-unit test-integration test-coverage bench lint

test:
	go test -v ./...

# Быстрые тесты без Redis
test-quick:
	@echo "Running quick tests without Redis..."
	go test -v -timeout=30s -run="^Test(ErrorTypes|DefaultOptions|Task|Backoff|NewQueue|NewWorker)" ./...

# Только unit тесты
test-unit:
	@echo "Running unit tests..."
	go test -v -timeout=30s -run="^Test(New|Task|Backoff|Default|Option|Error)" ./...

# Проверка сборки
test-build:
	@echo "Testing build..."
	go build -v ./...
	@echo "✅ Build successful"
	
test-integration:
	go test -v -run="^TestIntegration" ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench:
	go test -bench=. -benchmem ./...

lint:
	golangci-lint run

run-redis:
	docker run -d -p 6379:6379 --name test-redis redis:7-alpine

stop-redis:
	docker stop test-redis
	docker rm test-redis

test-setup: run-redis
	sleep 2

clean:
	go clean
	rm -f coverage.out coverage.html

# Помощь
help:
	@echo "Доступные команды:"
	@echo "  make test-setup    - Подготовка тестового окружения"
	@echo "  make test          - Запуск всех тестов"
	@echo "  make test-unit     - Запуск unit тестов"
	@echo "  make test-integration - Запуск интеграционных тестов"
	@echo "  make test-coverage - Тесты с покрытием кода"
	@echo "  make bench         - Бенчмарки"
	@echo "  make lint          - Проверка кодстайла"