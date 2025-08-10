#!/bin/bash

echo "🔍 Verifying MTG Pipeline Setup..."
echo "=================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check function
check_service() {
    local service=$1
    local port=$2
    local url=$3
    
    if docker ps | grep -q "$service"; then
        echo -e "${GREEN}✓${NC} $service is running"
        if [ ! -z "$port" ]; then
            if nc -z localhost $port 2>/dev/null; then
                echo -e "  ${GREEN}✓${NC} Port $port is accessible"
            else
                echo -e "  ${RED}✗${NC} Port $port is not accessible"
            fi
        fi
    else
        echo -e "${RED}✗${NC} $service is not running"
        return 1
    fi
}

# Check core services
echo ""
echo "📦 Core Services:"
check_service "zookeeper" 2181
check_service "kafka" 9092
check_service "postgres" 5432
check_service "minio" 9000

echo ""
echo "🔧 Processing Services:"
check_service "flink-jobmanager" 8081
check_service "ksqldb-server" 8088

echo ""
echo "🌐 UI Services:"
check_service "kafka-ui" 8080
check_service "web-ui" 8090
check_service "adminer" 8082

# Check Kafka topics
echo ""
echo "📊 Kafka Topics:"
topics=$(docker exec kafka kafka-topics --list --bootstrap-server localhost:9092 2>/dev/null | grep "mtg\." | wc -l)
if [ $topics -gt 0 ]; then
    echo -e "${GREEN}✓${NC} Found $topics MTG topics"
else
    echo -e "${RED}✗${NC} No MTG topics found"
fi

# Check KSQL streams
echo ""
echo "🔄 KSQL Streams:"
streams=$(docker exec ksqldb-cli ksql http://ksqldb-server:8088 --execute "SHOW STREAMS;" 2>/dev/null | grep -E "(DECK_VALUES_STREAM|CARDS_RAW)" | wc -l)
if [ $streams -gt 0 ]; then
    echo -e "${GREEN}✓${NC} Found $streams KSQL streams"
else
    echo -e "${RED}✗${NC} No KSQL streams found (run: make init-ksql)"
fi

# Check Flink jobs
echo ""
echo "⚡ Flink Jobs:"
if docker exec flink-jobmanager flink list 2>/dev/null | grep -q "RUNNING"; then
    echo -e "${GREEN}✓${NC} Flink jobs are running"
else
    echo -e "${RED}✗${NC} No Flink jobs running (run: make deploy-flink)"
fi

# Test web UI
echo ""
echo "🌐 Web UI Test:"
if curl -s http://localhost:8090/ | grep -q "MTG Data Pipeline"; then
    echo -e "${GREEN}✓${NC} Web UI is accessible at http://localhost:8090/"
else
    echo -e "${RED}✗${NC} Web UI is not accessible"
fi

# Test KSQL proxy
if curl -s -X POST http://localhost:8090/api/query \
    -H "Content-Type: application/vnd.ksql.v1+json" \
    -d '{"ksql": "LIST STREAMS;", "streamsProperties": {}}' 2>/dev/null | grep -q "stream"; then
    echo -e "${GREEN}✓${NC} KSQL proxy is working"
else
    echo -e "${RED}✗${NC} KSQL proxy is not working"
fi

echo ""
echo "=================================="
echo "📋 Summary:"
echo ""
echo "If all checks passed, you can:"
echo "  1. Ingest MTG data: make ingest-mtg"
echo "  2. Ingest decks: make ingest-decks"
echo "  3. Query deck values: make query-decks"
echo "  4. Access UI: http://localhost:8090/query-ui.html"
echo ""
echo "If some checks failed, run: make setup"