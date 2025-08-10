# MTG Data Pipeline & Deck Management System

A comprehensive Magic: The Gathering data platform with real-time processing, deck management, and portfolio valuation.

## 🎯 Features

- **Real-time Data Ingestion**: Automated daily collection from MTGJSON API
- **Event Streaming**: Apache Kafka for distributed event processing
- **Stream Processing**: Apache Flink for real-time data enrichment and deck valuation
- **Deck Management**: Parse and analyze MTG deck files with automatic value calculation
- **Query Interface**: Web-based UI for exploring cards, decks, and prices
- **Portfolio Tracking**: Monitor value of 19+ MTG decks (~$8,735 total)

## 🏗️ Architecture

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

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.21+ (for local development)
- 8GB+ RAM recommended
- Ports 8080-8090, 9000-9001, 5432 available

### Using Make (Recommended)

```bash
# Start entire development environment
make up

# Run complete setup (build + start + init)
make setup

# Ingest MTG data
make ingest-mtg

# Ingest deck files
make ingest-decks

# View logs
make logs

# Stop everything
make down
```

### Manual Setup

```bash
# 1. Clone repository
git clone https://github.com/lspecian/mtg.git
cd mtg

# 2. Start services
docker-compose up -d

# 3. Create Kafka topics
./data-ingestion/mtg-ingestor/scripts/create-deck-topics.sh

# 4. Build components
make build

# 5. Deploy Flink jobs
make deploy-flink

# 6. Run ingestion
make ingest-all
```

## 📊 Services & Ports

| Service | URL | Description |
|---------|-----|-------------|
| Kafka UI | http://localhost:8080 | Monitor Kafka topics and messages |
| Flink UI | http://localhost:8081 | View Flink job status |
| Adminer | http://localhost:8082 | PostgreSQL database UI |
| Query UI | http://localhost:8090/query-ui.html | Interactive data queries |
| Dashboard | http://localhost:8090 | MTG data dashboard |
| MinIO Console | http://localhost:9001 | S3-compatible storage UI |

## 📁 Project Structure

```
mtg/
├── decks/                      # MTG deck files (.deck format)
├── data-ingestion/
│   └── mtg-ingestor/          # Go-based ingestion service
│       ├── cmd/               # CLI applications
│       │   ├── main.go        # MTGJSON ingester
│       │   └── deck-ingester/ # Deck file processor
│       ├── internal/          # Core business logic
│       ├── flink/            # Java Flink processors
│       ├── ksql/             # KSQL query templates
│       ├── k8s/              # Kubernetes manifests
│       └── web-ui/           # Query interface
├── docker-compose.yml         # Local development stack
├── Makefile                  # Build automation
└── README.md                 # This file
```

## 🔧 Development

### Building Components

```bash
# Build all components
make build

# Build individually
make build-go        # Go services
make build-flink     # Flink processors
make build-docker    # Docker images
```

### Running Tests

```bash
# Run Go tests
make test

# Test deck ingestion (dry-run)
make test-decks
```

### Adding New Decks

1. Place `.deck` files in the `decks/` directory
2. Format: `<quantity> <card_name>` per line
3. Run `make ingest-decks` to process

### Querying Data

```bash
# List deck values
make query-decks

# View Kafka messages
make kafka-consume topic=mtg.decks

# Access KSQL CLI
make ksql-cli
```

## 📈 Current Statistics

- **Decks Tracked**: 19
- **Total Cards**: 1,652
- **Portfolio Value**: ~$8,735
- **Data Points**: 50M+ price records
- **Card Database**: 32,385 unique cards

## 🛠️ Troubleshooting

### Common Issues

1. **Port conflicts**: Ensure ports 8080-8090 are free
2. **Memory issues**: Allocate at least 8GB RAM to Docker
3. **Kafka connectivity**: Wait 30s after startup for services to initialize
4. **Large file warning**: Build artifacts are gitignored, run `make clean` if needed

### Useful Commands

```bash
# Check service health
make status

# View logs
make logs service=kafka

# Reset everything
make clean-all

# Rebuild from scratch
make reset
```

## 📚 Documentation

- [Architecture Details](CLAUDE.md)
- [Deck Ingestion Guide](data-ingestion/mtg-ingestor/DECK_INGESTION.md)
- [API Documentation](docs/API.md)
- [Deployment Guide](docs/DEPLOYMENT.md)

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## 📝 License

This project is licensed under the MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [MTGJSON](https://mtgjson.com/) for comprehensive MTG data
- [Apache Kafka](https://kafka.apache.org/) for event streaming
- [Apache Flink](https://flink.apache.org/) for stream processing
- [Confluent](https://www.confluent.io/) for KSQL and Kafka tools

## 📞 Support

For issues, questions, or suggestions:
- Open an [Issue](https://github.com/lspecian/mtg/issues)
- Check [Documentation](CLAUDE.md)
- Review [FAQ](docs/FAQ.md)

---

Built with ❤️ for the MTG community