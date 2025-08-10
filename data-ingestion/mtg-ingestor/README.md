# MTG Data Ingestion Pipeline

## Overview
A modern data pipeline for ingesting Magic: The Gathering card data from MTGJSON, processing it through Kafka and Flink, and storing it in PostgreSQL and S3.

## Architecture Components

### 1. Go Ingestion Service
- Fetches data from MTGJSON API
- Publishes events to Kafka topics
- Runs as a Kubernetes CronJob (daily at 3 AM UTC)

### 2. Apache Kafka
- Event streaming platform
- Topics:
  - `mtg.cards` - Individual card events
  - `mtg.sets` - Set information
  - `mtg.prices` - Pricing data
  - `mtg.statistics` - Aggregated statistics

### 3. Apache Flink
- Stream processing for real-time analytics
- Enriches card data
- Calculates statistics
- Outputs to S3 (Parquet) and PostgreSQL

### 4. PostgreSQL
- Persistent storage for card data
- Normalized schema with indexes
- Views for common queries

### 5. S3 (MinIO locally)
- Object storage for raw and processed data
- Parquet format for analytics

## Local Development

### Prerequisites
- Docker and Docker Compose
- Go 1.21+
- Java 11 (for Flink development)
- kubectl (for Kubernetes deployment)

### Quick Start

1. **Start infrastructure**:
```bash
docker-compose up -d
```

2. **Check services**:
- Kafka UI: http://localhost:8080
- Flink UI: http://localhost:8081
- Adminer (DB): http://localhost:8082
- MinIO Console: http://localhost:9001

3. **Run ingestion manually**:
```bash
docker-compose --profile ingest up mtg-ingestor
```

4. **Build Flink job**:
```bash
cd flink
mvn clean package
```

5. **Submit Flink job**:
```bash
docker cp target/mtg-flink-processor-1.0.0.jar flink-jobmanager:/opt/flink/
docker exec flink-jobmanager flink run /opt/flink/mtg-flink-processor-1.0.0.jar
```

## Kubernetes Deployment

1. **Create namespace**:
```bash
kubectl create namespace mtg-data
```

2. **Deploy secrets**:
```bash
kubectl apply -f k8s/secrets.yaml
```

3. **Deploy CronJob**:
```bash
kubectl apply -f k8s/cronjob.yaml
```

4. **Check job status**:
```bash
kubectl get cronjobs -n mtg-data
kubectl get jobs -n mtg-data
```

## Configuration

### Environment Variables
- `KAFKA_BROKERS`: Kafka broker addresses
- `POSTGRES_PASSWORD`: Database password
- `AWS_ACCESS_KEY_ID`: S3 access key
- `AWS_SECRET_ACCESS_KEY`: S3 secret key

### Config File
See `configs/config.yaml` for detailed configuration options.

## Monitoring

### Kafka Topics
```bash
# List topics
docker exec kafka kafka-topics --list --bootstrap-server localhost:9092

# Consume messages
docker exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic mtg.cards \
  --from-beginning \
  --max-messages 10
```

### PostgreSQL Queries
```sql
-- Count cards by rarity
SELECT rarity, COUNT(*) 
FROM cards 
GROUP BY rarity;

-- Recent price updates
SELECT * 
FROM card_prices 
ORDER BY created_at DESC 
LIMIT 10;
```

### Flink Jobs
Access Flink Web UI at http://localhost:8081 to monitor:
- Running jobs
- Task managers
- Checkpoints
- Metrics

## Troubleshooting

### Common Issues

1. **Kafka connection failed**:
   - Check if Kafka is running: `docker-compose ps`
   - Verify network connectivity

2. **PostgreSQL connection error**:
   - Check credentials in environment
   - Ensure database is initialized

3. **Flink job failures**:
   - Check task manager logs: `docker logs flink-taskmanager`
   - Verify checkpoint directory permissions

## Development

### Adding New Data Sources
1. Update models in `internal/models/`
2. Add fetcher methods in `internal/fetcher/`
3. Update Kafka producer in `internal/kafka/`
4. Modify Flink processor accordingly

### Testing
```bash
# Run Go tests
go test ./...

# Integration test with local stack
docker-compose up -d
go run cmd/main.go
```

## Production Considerations

1. **Scaling**:
   - Increase Kafka partitions for higher throughput
   - Add more Flink task managers
   - Use PostgreSQL read replicas

2. **Security**:
   - Enable Kafka SSL/SASL
   - Use IAM roles for S3 access
   - Implement network policies in Kubernetes

3. **Monitoring**:
   - Deploy Prometheus + Grafana
   - Enable Flink metrics
   - Set up alerting for job failures

4. **Backup**:
   - Regular PostgreSQL backups
   - S3 versioning and lifecycle policies
   - Kafka topic retention configuration