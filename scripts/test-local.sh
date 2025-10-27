#!/bin/bash

set -e  # Остановить при первой ошибке

echo "🔧 Running basic tests without Redis..."

# Запускаем тесты по одному для лучшей диагностики
echo "1. Running error tests..."
go test -v -timeout=30s -run="^TestErrorTypes" ./...

echo "2. Running options tests..."
go test -v -timeout=30s -run="^TestDefaultOptions" ./...

echo "3. Running task tests..."
go test -v -timeout=30s -run="^Test(Task|NewTask)" ./...

echo "4. Running backoff tests..."
go test -v -timeout=30s -run="^Test(Backoff|Constant|Exponential|Jitter|Composite)" ./...

echo "5. Running queue creation tests..."
go test -v -timeout=30s -run="^TestNewQueue" ./...

echo "6. Running worker creation tests..."
go test -v -timeout=30s -run="^TestNewWorker" ./...

echo "✅ All basic tests passed!"

echo "🔨 Building package..."
go build -v ./...

if [ $? -eq 0 ]; then
    echo "✅ Build successful"
else
    echo "❌ Build failed"
    exit 1
fi

echo "🎉 All checks passed!"