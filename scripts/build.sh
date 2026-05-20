#!/bin/bash

# Сборка единого исполняемого файла с поддержкой подкоманд

echo "🔨 Building SSH Sync Service with Cobra CLI..."

go build -o bin/ssh-sync-service main.go

echo "✅ Build complete. Run:"
echo "  ./bin/ssh-sync-service              # Монолит"
echo "  ./bin/ssh-sync-service backend      # Только API"
echo "  ./bin/ssh-sync-service scheduler    # Планировщик"
echo "  ./bin/ssh-sync-service worker       # Worker"