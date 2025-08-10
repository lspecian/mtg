#!/bin/bash

echo "🔧 Initializing KSQL streams..."

# Wait for KSQL to be ready
echo "Waiting for KSQL to be ready..."
for i in {1..30}; do
    if docker exec ksqldb-cli ksql http://ksqldb-server:8088 --execute "SHOW STREAMS;" 2>/dev/null | grep -q "Stream Name"; then
        echo "✅ KSQL is ready"
        break
    fi
    echo -n "."
    sleep 2
done

# Create streams
echo "Creating KSQL streams..."

# Deck values stream
docker exec -i ksqldb-cli ksql http://ksqldb-server:8088 <<EOF 2>/dev/null | grep -v WARNING || true
CREATE STREAM IF NOT EXISTS deck_values_stream (
    eventType VARCHAR,
    eventId VARCHAR,
    timestamp BIGINT,
    source VARCHAR,
    version VARCHAR,
    data STRUCT<
        deck_id VARCHAR,
        deck_name VARCHAR,
        total_cards INT,
        total_value DOUBLE,
        calculated_at BIGINT
    >
) WITH (
    KAFKA_TOPIC='mtg.deck-values',
    VALUE_FORMAT='JSON'
);
EOF

# Cards stream
docker exec -i ksqldb-cli ksql http://ksqldb-server:8088 <<EOF 2>/dev/null | grep -v WARNING || true
CREATE STREAM IF NOT EXISTS cards_raw (
    eventType VARCHAR,
    eventId VARCHAR,
    timestamp BIGINT,
    source VARCHAR,
    version VARCHAR,
    card STRUCT<
        uuid VARCHAR,
        name VARCHAR,
        type VARCHAR,
        rarity VARCHAR,
        setCode VARCHAR
    >
) WITH (
    KAFKA_TOPIC='mtg.cards',
    VALUE_FORMAT='JSON'
);
EOF

echo "✅ KSQL streams initialized"

# Show created streams
echo ""
echo "📊 Available KSQL streams:"
docker exec ksqldb-cli ksql http://ksqldb-server:8088 --execute "SHOW STREAMS;" 2>/dev/null | grep -v "ksql>" | grep -v WARNING || true