#!/bin/bash

echo "Creating Kafka topics for deck processing..."

# Wait for Kafka to be ready
docker exec kafka kafka-topics --bootstrap-server localhost:9092 --list > /dev/null 2>&1
while [ $? -ne 0 ]; do
    echo "Waiting for Kafka to be ready..."
    sleep 2
    docker exec kafka kafka-topics --bootstrap-server localhost:9092 --list > /dev/null 2>&1
done

echo "Kafka is ready. Creating topics..."

# Create deck-related topics
docker exec kafka kafka-topics --create --if-not-exists \
    --bootstrap-server localhost:9092 \
    --partitions 2 \
    --replication-factor 1 \
    --topic mtg.decks

docker exec kafka kafka-topics --create --if-not-exists \
    --bootstrap-server localhost:9092 \
    --partitions 3 \
    --replication-factor 1 \
    --topic mtg.deck-cards

docker exec kafka kafka-topics --create --if-not-exists \
    --bootstrap-server localhost:9092 \
    --partitions 2 \
    --replication-factor 1 \
    --topic mtg.deck-values

echo "Topics created successfully!"
echo "Listing all topics:"
docker exec kafka kafka-topics --list --bootstrap-server localhost:9092