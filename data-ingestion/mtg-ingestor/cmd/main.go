package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mtg/mtg-ingestor/internal/fetcher"
	"github.com/mtg/mtg-ingestor/internal/kafka"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	if err := loadConfig(); err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Set log level
	level, err := logrus.ParseLevel(viper.GetString("app.log_level"))
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	logger.Info("Starting MTG data ingestion job")

	// Initialize MTG fetcher
	mtgFetcher := fetcher.NewMTGFetcher(logger)

	// Initialize Kafka producer
	kafkaProducer, err := kafka.NewProducer(kafka.ProducerConfig{
		Brokers:     viper.GetString("kafka.brokers"),
		CardsTopic:  viper.GetString("kafka.topics.cards"),
		SetsTopic:   viper.GetString("kafka.topics.sets"),
		PricesTopic: viper.GetString("kafka.topics.prices"),
		Logger:      logger,
	})
	if err != nil {
		logger.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	// Start ingestion process
	startTime := time.Now()

	// Fetch and publish sets data
	logger.Info("Fetching MTG sets data...")
	sets, err := mtgFetcher.FetchAllSets()
	if err != nil {
		logger.Errorf("Failed to fetch sets: %v", err)
	} else {
		logger.Infof("Publishing %d sets to Kafka", len(sets))
		publishedSets := 0
		for _, set := range sets {
			if err := kafkaProducer.PublishSet(set); err != nil {
				logger.Errorf("Failed to publish set %s: %v", set.Code, err)
			} else {
				publishedSets++
				if publishedSets%100 == 0 {
					logger.Infof("Published %d/%d sets", publishedSets, len(sets))
				}
			}
		}
		logger.Infof("Successfully published %d sets", publishedSets)
	}

	// Fetch and publish atomic cards
	logger.Info("Fetching atomic cards data...")
	cards, err := mtgFetcher.FetchAtomicCards()
	if err != nil {
		logger.Errorf("Failed to fetch atomic cards: %v", err)
	} else {
		logger.Infof("Publishing %d cards to Kafka", len(cards))
		publishedCards := 0
		for _, card := range cards {
			if err := kafkaProducer.PublishCard(card); err != nil {
				logger.Errorf("Failed to publish card %s: %v", card.Name, err)
			} else {
				publishedCards++
				if publishedCards%1000 == 0 {
					logger.Infof("Published %d/%d cards", publishedCards, len(cards))
				}
			}
		}
		logger.Infof("Successfully published %d cards", publishedCards)
	}

	// Fetch and publish prices
	logger.Info("Fetching price data...")
	prices, err := mtgFetcher.FetchPrices()
	if err != nil {
		logger.Errorf("Failed to fetch prices: %v", err)
	} else {
		logger.Infof("Publishing %d individual price records to Kafka", len(prices))
		publishedPrices := 0
		for _, price := range prices {
			if err := kafkaProducer.PublishPrice(price); err != nil {
				logger.Errorf("Failed to publish price: %v", err)
			} else {
				publishedPrices++
				if publishedPrices%1000 == 0 {
					logger.Infof("Published %d/%d prices", publishedPrices, len(prices))
				}
			}
		}
		logger.Infof("Successfully published %d price records", publishedPrices)
	}

	// Flush any remaining messages
	remaining := kafkaProducer.Flush(30000)
	if remaining > 0 {
		logger.Warnf("%d messages were not delivered", remaining)
	}

	duration := time.Since(startTime)
	logger.Infof("Ingestion completed in %v", duration)
}

func loadConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/app/configs")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Enable environment variable override
	viper.AutomaticEnv()
	viper.SetEnvPrefix("MTG")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; use defaults and environment variables
			setDefaults()
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	return nil
}

func setDefaults() {
	viper.SetDefault("app.name", "mtg-ingestor")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.log_level", "info")

	viper.SetDefault("kafka.brokers", getEnvOrDefault("KAFKA_BROKERS", "localhost:9092"))
	viper.SetDefault("kafka.topics.cards", "mtg.cards")
	viper.SetDefault("kafka.topics.sets", "mtg.sets")
	viper.SetDefault("kafka.topics.prices", "mtg.prices")

	viper.SetDefault("postgres.host", getEnvOrDefault("POSTGRES_HOST", "localhost"))
	viper.SetDefault("postgres.port", 5432)
	viper.SetDefault("postgres.database", "mtg")
	viper.SetDefault("postgres.user", "mtg_user")
	viper.SetDefault("postgres.ssl_mode", "disable")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}