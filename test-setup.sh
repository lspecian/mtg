#!/bin/bash

echo "🧪 Testing MTG Setup from Scratch"
echo "=================================="

# Test if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

echo "✅ Docker is running"

# Test if docker-compose exists
if ! command -v docker-compose &> /dev/null; then
    if ! command -v docker compose &> /dev/null; then
        echo "❌ docker-compose is not installed"
        exit 1
    fi
    echo "✅ Docker Compose (docker compose) found"
else
    echo "✅ docker-compose found"
fi

# Test if make exists
if ! command -v make &> /dev/null; then
    echo "❌ make is not installed"
    exit 1
fi
echo "✅ make found"

# Check ports
for port in 8080 8081 8082 8088 8090 9000 9001 5432; do
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "⚠️  Port $port is already in use"
    fi
done

echo ""
echo "📝 Running make setup..."
echo ""

# Run setup
make setup

echo ""
echo "🔍 Verifying setup..."
sleep 10

# Check services
if docker ps | grep -q "web-ui"; then
    echo "✅ Web UI is running"
else
    echo "❌ Web UI is not running"
fi

if docker ps | grep -q "kafka"; then
    echo "✅ Kafka is running"
else
    echo "❌ Kafka is not running"
fi

if docker ps | grep -q "flink-jobmanager"; then
    echo "✅ Flink is running"
else
    echo "❌ Flink is not running"
fi

# Test web UI
if curl -s http://localhost:8090/ | grep -q "MTG"; then
    echo "✅ Web UI is accessible at http://localhost:8090/"
else
    echo "❌ Web UI is not accessible"
fi

echo ""
echo "=================================="
echo "Setup test complete!"
echo ""
echo "Next steps:"
echo "  1. Ingest data: make ingest-mtg"
echo "  2. Ingest decks: make ingest-decks"
echo "  3. Access UI: http://localhost:8090/"