#!/bin/bash
set -e

echo "🚀 BlockMesh Self-Hosted Installer"

if ! command -v docker &> /dev/null; then
    echo "❌ Docker is required but not installed."
    exit 1
fi

if [ ! -f .env ]; then
    echo "📝 Creating .env file..."
    cp .env.example .env
    echo "⚠️  Please edit .env and set your passwords and domain, then re-run."
    exit 0
fi

echo "📦 Starting services..."
docker compose pull
docker compose up -d

echo "✅ BlockMesh is running!"
echo "   Dashboard: http://localhost:3000"
echo "   Gateway:   http://localhost:8080"
echo "   Admin API: http://localhost:8081"
