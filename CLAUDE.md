# MTG Data Pipeline and Deck Management System

## Project Overview
A comprehensive Magic: The Gathering (MTG) data platform featuring:
- Real-time data ingestion from MTGJSON
- Event-driven architecture with Kafka
- Stream processing with Apache Flink
- Persistent storage in PostgreSQL and S3
- Deck management system with value calculation
- Portfolio tracking for 19 MTG decks (~$8,735 total value)

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
│   MTGJSON   │────▶│ Go Ingestor │────▶│    Kafka     │
│     API     │     │  (K8s Job)  │     │   Topics     │
└─────────────┘     └─────────────┘     └──────────────┘
                                                │
┌─────────────┐     ┌─────────────┐            │
│ Deck Files  │────▶│Deck Ingester│────────────┤
│   (.deck)   │     │     (Go)    │            │
└─────────────┘     └─────────────┘            ▼
                                        ┌──────────────┐
                                        │    Flink     │
                                        │  Processing  │
                                        └──────────────┘
                                           │        │
                                           ▼        ▼
                                    ┌──────────┐ ┌─────────┐
                                    │PostgreSQL│ │   S3    │
                                    └──────────┘ └─────────┘
```

## Project Structure

```
mtg/
├── decks/                          # MTG deck files
│   ├── *.deck                      # Deck lists
│   └── *.txt                       # Alternative formats
├── data-ingestion/
│   ├── mtg-ingestor/               # Go-based ingestion service
│   │   ├── cmd/
│   │   │   ├── main.go             # MTGJSON ingester
│   │   │   └── deck-ingester/      # Deck file ingester
│   │   ├── internal/
│   │   │   ├── fetcher/            # MTGJSON API client
│   │   │   ├── kafka/              # Kafka producer
│   │   │   ├── deck/               # Deck parsing logic
│   │   │   └── models/             # Data models
│   │   ├── configs/                # Configuration files
│   │   ├── k8s/                    # Kubernetes manifests
│   │   ├── flink/                  # Flink job (Java)
│   │   ├── sql/                    # Database schema
│   │   └── Dockerfile              # Container image
│   └── mtg_json/                   # Legacy Rust implementation
├── docker-compose.yml              # Local development stack
└── data/                           # Data storage

```

## Technology Stack

### Core Technologies
- **Go**: Primary language for data ingestion
- **Apache Kafka**: Event streaming platform
- **Apache Flink**: Stream processing engine
- **PostgreSQL**: Relational database
- **S3/MinIO**: Object storage
- **Kubernetes**: Container orchestration
- **Docker**: Containerization

### Data Formats
- **JSON**: Source data from MTGJSON
- **Avro/JSON**: Kafka message format
- **Parquet**: Analytical storage in S3
- **SQL**: Structured storage in PostgreSQL

## Key Components

### 1. Go Ingestion Service (`/data-ingestion/mtg-ingestor/`)
- Fetches data from MTGJSON API endpoints
- Publishes events to Kafka topics
- Runs as Kubernetes CronJob (daily at 3 AM UTC)
- Handles:
  - Card data (`AllSets.json`)
  - Atomic cards (`AtomicCards.json`)
  - Price data (`AllPrices.json`)

### 2. Kafka Topics
- `mtg.cards`: Individual card events
- `mtg.sets`: Set information
- `mtg.prices`: Pricing updates
- `mtg.statistics`: Aggregated metrics
- `mtg.decks`: Complete deck information
- `mtg.deck-cards`: Individual deck card events
- `mtg.deck-values`: Calculated deck values

### 3. Flink Processing Pipeline
- **MTGDataProcessor**: Enriches card data with calculated fields
- **DeckValueProcessor**: Calculates total deck values by joining with prices
- Outputs to:
  - PostgreSQL for querying
  - S3 in Parquet format for analytics
  - Kafka topics for real-time streaming

### 4. PostgreSQL Schema
- Normalized tables for cards, sets, prices
- Support for deck management
- Optimized indexes for common queries
- Views for simplified access

### 5. Deck Management (`/decks/`)
- Plain text deck files (`.deck` format)
- Format: `<quantity> <card name>`
- 19 decks currently tracked:
  - 13 Commander decks (100 cards)
  - 6 Variable-size decks
- Total portfolio value: ~$8,735

## Local Development

### Quick Start
```bash
# Start all services
docker-compose up -d

# Run MTGJSON ingestion
docker-compose --profile ingest up mtg-ingestor

# Run deck ingestion
docker-compose run --rm --entrypoint /app/deck-ingester deck-ingestor -dir /decks

# Access services
# Kafka UI: http://localhost:8080
# Flink UI: http://localhost:8081
# Adminer: http://localhost:8082
# MinIO: http://localhost:9001
# Query UI: http://localhost:8090/query-ui.html
# Dashboard: http://localhost:8090/
```

### Building Components
```bash
# Build Go service
cd data-ingestion/mtg-ingestor
go build -o mtg-ingestor cmd/main.go

# Build Flink job
cd flink
mvn clean package

# Build Docker image
docker build -t mtg-ingestor:latest .
```

## Kubernetes Deployment

### Deploy to Cluster
```bash
# Create namespace
kubectl create namespace mtg-data

# Apply configurations
kubectl apply -f data-ingestion/mtg-ingestor/k8s/

# Check status
kubectl get cronjobs -n mtg-data
kubectl get pods -n mtg-data
```

## Data Querying

### PostgreSQL Queries
```sql
-- Card search
SELECT * FROM cards WHERE name ILIKE '%dragon%';

-- Deck composition
SELECT * FROM v_deck_composition WHERE deck_id = 1;

-- Price trends
SELECT card_uuid, date, price 
FROM card_prices 
WHERE source = 'tcgplayer' 
ORDER BY date DESC;
```

### Kafka Message Consumption
```bash
# Consume card events
kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic mtg.cards \
  --from-beginning
```

## Configuration

### Environment Variables
- `KAFKA_BROKERS`: Kafka broker addresses
- `POSTGRES_HOST`: Database host
- `POSTGRES_PASSWORD`: Database password
- `AWS_ACCESS_KEY_ID`: S3 access key
- `AWS_SECRET_ACCESS_KEY`: S3 secret key

### Config Files
- `configs/config.yaml`: Application configuration
- `k8s/cronjob.yaml`: Kubernetes job schedule
- `docker-compose.yml`: Local service definitions

## Monitoring & Operations

### Health Checks
- Kafka UI for topic monitoring
- Flink Web UI for job status
- PostgreSQL metrics via Adminer
- MinIO console for S3 storage

### Logging
- Structured JSON logging
- Log levels: DEBUG, INFO, WARN, ERROR
- Centralized in production via Kubernetes

## Development Guidelines

### Adding New Data Sources
1. Update models in `internal/models/`
2. Add fetcher methods in `internal/fetcher/`
3. Update Kafka producer
4. Modify Flink processor
5. Update database schema if needed

### Testing
```bash
# Unit tests
go test ./...

# Integration test
docker-compose up -d
go run cmd/main.go
```

## Production Considerations

### Scaling
- Kafka: Add partitions for throughput
- Flink: Increase parallelism and task slots
- PostgreSQL: Read replicas, partitioning
- Kubernetes: HPA for auto-scaling

### Security
- Enable Kafka SSL/SASL authentication
- Use Kubernetes secrets for credentials
- Implement network policies
- Regular security updates

### Reliability
- Kafka replication factor ≥ 3
- Flink checkpointing enabled
- PostgreSQL backups and WAL archiving
- S3 versioning and lifecycle policies

## Troubleshooting

### Common Issues
1. **Kafka connectivity**: Check broker addresses and network
2. **Flink checkpoints**: Verify storage permissions
3. **PostgreSQL connections**: Check connection pool settings
4. **S3 uploads**: Verify credentials and bucket policies

### Debug Commands
```bash
# Check pod logs
kubectl logs -n mtg-data <pod-name>

# Kafka topic details
kafka-topics --describe --topic mtg.cards \
  --bootstrap-server localhost:9092

# PostgreSQL connections
SELECT * FROM pg_stat_activity;

# Flink job status
curl http://localhost:8081/jobs
```

## Recent Achievements (2025-08-10)
- ✅ Implemented deck ingestion pipeline for 19 decks
- ✅ Created Flink job for real-time deck value calculation
- ✅ Built web-based query interface with KSQL integration
- ✅ Successfully processing 1,652 cards across all decks
- ✅ Deck portfolio valued at ~$8,735

## Future Enhancements
- GraphQL API for deck queries
- Machine learning for card recommendations
- Real-time deck validation with actual market prices
- Tournament result integration
- Mobile application support
- Advanced analytics dashboard
- Deck optimization based on budget constraints
- Historical value tracking for portfolio management