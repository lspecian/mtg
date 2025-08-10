# MTG Deck Ingestion & Value Calculation

## Overview
The deck ingestion system processes Magic: The Gathering deck files, publishes them to Kafka, and calculates their total value using Apache Flink by matching card prices.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│ Deck Files  │────▶│ Go Ingester  │────▶│   Kafka     │
│   (.deck)   │     │              │     │   Topics    │
└─────────────┘     └──────────────┘     └─────────────┘
                                                │
                                         ┌──────┴──────┐
                                         ▼             ▼
                                   mtg.decks    mtg.deck-cards
                                         │             │
                                         └──────┬──────┘
                                                ▼
                                         ┌──────────────┐
                                         │    Flink     │
                                         │  Processor   │
                                         └──────────────┘
                                                │
                                         Joins with prices
                                                │
                                                ▼
                                         mtg.deck-values
```

## Components

### 1. Deck File Format
- Plain text files with `.deck` or `.deck.txt` extension
- Format: `<quantity> <card_name>`
- Example:
```
1 Lightning Bolt
4 Counterspell
2 Black Lotus
```

### 2. Go Deck Ingester
- **Location**: `cmd/deck-ingester/main.go`
- **Package**: `internal/deck/ingester.go`
- Reads deck files from a directory
- Parses card quantities and names
- Publishes events to Kafka

### 3. Kafka Topics
- `mtg.decks`: Complete deck information
- `mtg.deck-cards`: Individual card entries for processing
- `mtg.deck-values`: Calculated deck values

### 4. Flink Deck Value Processor
- **Location**: `flink/src/main/java/com/mtg/flink/DeckValueProcessor.java`
- Consumes deck-card events
- Joins with price data from `mtg.prices`
- Calculates total deck value
- Outputs to `mtg.deck-values`

## Usage

### Local Development

#### 1. Start Infrastructure
```bash
# Start Kafka, PostgreSQL, MinIO, Flink
docker-compose up -d

# Create deck-related topics
./scripts/create-deck-topics.sh
```

#### 2. Run Deck Ingestion
```bash
# Build and run locally
go run cmd/deck-ingester/main.go -dir ../../decks -dry-run

# Run with Docker
docker-compose --profile deck-ingest up deck-ingestor
```

#### 3. Deploy Flink Job
```bash
# Build Flink job
cd flink
mvn clean package

# Submit to Flink
docker cp target/mtg-flink-processor-1.0.0.jar flink-jobmanager:/tmp/
docker exec flink-jobmanager flink run /tmp/mtg-flink-processor-1.0.0.jar \
  --class com.mtg.flink.DeckValueProcessor
```

### Production Deployment

#### Kubernetes CronJob
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: deck-ingester
  namespace: mtg-data
spec:
  schedule: "0 4 * * *"  # Daily at 4 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: deck-ingester
            image: mtg-ingestor:latest
            command: ["/app/deck-ingester"]
            args: ["-dir", "/decks"]
            env:
            - name: KAFKA_BROKERS
              value: "kafka:9092"
            volumeMounts:
            - name: decks
              mountPath: /decks
          volumes:
          - name: decks
            persistentVolumeClaim:
              claimName: deck-storage
          restartPolicy: OnFailure
```

## Event Schemas

### Deck Event
```json
{
  "eventType": "deck.ingested",
  "eventId": "uuid",
  "timestamp": "2025-08-10T00:00:00Z",
  "source": "deck-ingester",
  "version": "v1",
  "data": {
    "id": "deck-uuid",
    "name": "My Deck Name",
    "file_path": "/decks/my-deck.deck",
    "cards": [
      {"quantity": 1, "name": "Lightning Bolt"},
      {"quantity": 4, "name": "Counterspell"}
    ],
    "total_cards": 100,
    "unique_cards": 75,
    "ingested_at": "2025-08-10T00:00:00Z"
  }
}
```

### Deck Card Event
```json
{
  "eventType": "deck.card",
  "eventId": "uuid",
  "timestamp": "2025-08-10T00:00:00Z",
  "source": "deck-ingester",
  "version": "v1",
  "data": {
    "deck_id": "deck-uuid",
    "deck_name": "My Deck Name",
    "card_name": "Lightning Bolt",
    "quantity": 1
  }
}
```

### Deck Value Event
```json
{
  "eventType": "deck.value.calculated",
  "eventId": "uuid",
  "timestamp": "2025-08-10T00:00:00Z",
  "source": "flink-deck-processor",
  "version": "v1",
  "data": {
    "deck_id": "deck-uuid",
    "deck_name": "My Deck Name",
    "total_cards": 100,
    "total_value": 1234.56,
    "calculated_at": 1723248000000
  }
}
```

## Querying Deck Values

### KSQL Queries

```sql
-- Create stream for deck values
CREATE STREAM deck_values_stream (
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

-- Find most expensive decks
SELECT 
  data->deck_name AS deck_name,
  data->total_value AS value
FROM deck_values_stream
WHERE data->total_value > 1000
EMIT CHANGES;

-- Average deck value
SELECT 
  COUNT(*) AS deck_count,
  AVG(data->total_value) AS avg_value,
  MAX(data->total_value) AS max_value
FROM deck_values_stream
EMIT CHANGES;
```

### Web UI Query
Access the query interface at http://localhost:8090/query-ui.html to:
- View deck values in real-time
- Compare deck prices
- Track value changes over time

## Monitoring

### Kafka UI
- URL: http://localhost:8080
- Monitor topics: `mtg.decks`, `mtg.deck-cards`, `mtg.deck-values`

### Flink UI
- URL: http://localhost:8081
- Check job: DeckValueProcessor
- Monitor throughput and latency

## Troubleshooting

### Common Issues

1. **No deck values appearing**
   - Check if price data exists in `mtg.prices`
   - Verify Flink job is running
   - Check Kafka topic lag

2. **Incorrect card matching**
   - Card names must match exactly
   - Check for special characters or editions

3. **Missing decks**
   - Verify deck file format
   - Check ingester logs for parsing errors

### Debug Commands

```bash
# Check deck topics
docker exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic mtg.decks \
  --from-beginning \
  --max-messages 5

# View deck values
docker exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic mtg.deck-values \
  --from-beginning

# Check Flink logs
docker logs flink-taskmanager

# Test deck parsing
go run test-deck-ingester.go -dir ../../decks
```

## Future Enhancements
- Real-time price updates for decks
- Historical value tracking
- Deck optimization suggestions based on budget
- Format legality checking
- Card availability alerts
- Deck sharing and comparison features