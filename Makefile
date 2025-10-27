# Makefile для TaskQueue

.PHONY: test test-unit test-integration test-coverage bench lint

# Запуск всех тестов
test:
	go test -v ./...

# Запуск только unit тестов
test-unit:
	go test -v -run="^Test(New|Task|Queue|Worker|Backoff)" ./...

# Запуск интеграционных тестов
test-integration:
	go test -v -run="^TestIntegration" ./...

# Запуск тестов с покрытием
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Бенчмарки
bench:
	go test -bench=. -benchmem ./...

# Линтинг
lint:
	golangci-lint run

# Запуск Redis для тестов
run-redis:
	docker run -d -p 6379:6379 --name test-redis redis:7-alpine

# Остановка Redis
stop-redis:
	docker stop test-redis
	docker rm test-redis

# Подготовка тестового окружения
test-setup: run-redis
	sleep 2

# Очистка
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