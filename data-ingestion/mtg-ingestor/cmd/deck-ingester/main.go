package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func main() {
	var (
		decksDir   = flag.String("dir", "/decks", "Directory containing deck files")
		configPath = flag.String("config", "configs/config.yaml", "Path to config file")
		dryRun     = flag.Bool("dry-run", false, "Dry run mode - don't publish to Kafka")
	)
	flag.Parse()

	// Setup logging
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// Load configuration
	viper.SetConfigFile(*configPath)
	viper.SetDefault("kafka.brokers", []string{"kafka:29092"})
	
	if err := viper.ReadInConfig(); err != nil {
		logger.Warnf("Could not read config file: %v, using defaults", err)
	}

	// Ingest all deck files
	logger.Infof("Starting deck ingestion from directory: %s", *decksDir)
	
	files, err := filepath.Glob(filepath.Join(*decksDir, "*.deck"))
	if err != nil {
		logger.WithError(err).Fatal("Failed to list deck files")
	}

	// Also check for .txt deck files
	txtFiles, err := filepath.Glob(filepath.Join(*decksDir, "*.deck.txt"))
	if err == nil {
		files = append(files, txtFiles...)
	}

	logger.Infof("Found %d deck files to process", len(files))

	var decks []map[string]interface{}
	for _, filePath := range files {
		deck, err := ingestDeckFile(filePath, logger)
		if err != nil {
			logger.WithError(err).Errorf("Failed to ingest deck file: %s", filePath)
			continue
		}
		decks = append(decks, deck)
	}

	logger.Infof("Successfully ingested %d decks", len(decks))

	if *dryRun {
		logger.Info("Dry run mode - skipping Kafka publishing")
		for _, d := range decks {
			jsonData, _ := json.MarshalIndent(d, "", "  ")
			fmt.Printf("Deck: %s\n%s\n\n", d["name"], string(jsonData))
		}
		return
	}

	// Create Kafka producer
	brokers := viper.GetStringSlice("kafka.brokers")
	if len(brokers) == 0 {
		brokers = []string{"kafka:29092"}
	}

	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers[0],
	})
	if err != nil {
		logger.WithError(err).Fatal("Failed to create Kafka producer")
	}
	defer producer.Close()

	// Publish deck events to Kafka
	publishedCount := 0
	cardEventCount := 0

	for _, deck := range decks {
		// Publish main deck event
		deckEvent := createDeckEvent(deck)
		if err := publishEvent(producer, "mtg.decks", deckEvent, logger); err != nil {
			logger.WithError(err).Errorf("Failed to publish deck event for: %s", deck["name"])
			continue
		}
		publishedCount++

		// Publish individual card events for Flink processing
		if cards, ok := deck["cards"].([]map[string]interface{}); ok {
			for _, card := range cards {
				cardEvent := createDeckCardEvent(deck["id"].(string), deck["name"].(string), card)
				if err := publishEvent(producer, "mtg.deck-cards", cardEvent, logger); err != nil {
					logger.WithError(err).Error("Failed to publish deck card event")
					continue
				}
				cardEventCount++
			}
		}

		// Small delay to avoid overwhelming Kafka
		time.Sleep(10 * time.Millisecond)
	}

	// Flush remaining messages
	producer.Flush(15 * 1000)

	logger.Infof("Published %d deck events and %d card events to Kafka", publishedCount, cardEventCount)
}

func ingestDeckFile(filePath string, logger *logrus.Logger) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	deck := map[string]interface{}{
		"id":          uuid.New().String(),
		"name":        extractDeckName(filePath),
		"file_path":   filePath,
		"cards":       []map[string]interface{}{},
		"ingested_at": time.Now(),
	}

	totalCards := 0
	cards := []map[string]interface{}{}
	
	lines := string(content)
	for _, line := range splitLines(lines) {
		line = trimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || hasPrefix(line, "//") || hasPrefix(line, "#") {
			continue
		}

		// Parse quantity and card name
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			quantity, err := strconv.Atoi(parts[0])
			if err == nil && quantity > 0 {
				cardName := trimSpace(parts[1])
				if cardName != "" {
					cards = append(cards, map[string]interface{}{
						"quantity": quantity,
						"name":     cardName,
					})
					totalCards += quantity
				}
			}
		}
	}

	deck["cards"] = cards
	deck["total_cards"] = totalCards
	deck["unique_cards"] = len(cards)

	logger.Infof("Ingested deck '%s': %d unique cards, %d total cards", 
		deck["name"], deck["unique_cards"], deck["total_cards"])

	return deck, nil
}

func createDeckEvent(deck map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"eventType": "deck.ingested",
		"eventId":   uuid.New().String(),
		"timestamp": time.Now(),
		"source":    "deck-ingester",
		"version":   "v1",
		"data":      deck,
	}
}

func createDeckCardEvent(deckId, deckName string, card map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"eventType": "deck.card",
		"eventId":   uuid.New().String(),
		"timestamp": time.Now(),
		"source":    "deck-ingester",
		"version":   "v1",
		"data": map[string]interface{}{
			"deck_id":   deckId,
			"deck_name": deckName,
			"card_name": card["name"],
			"quantity":  card["quantity"],
		},
	}
}

func publishEvent(producer *kafka.Producer, topic string, event map[string]interface{}, logger *logrus.Logger) error {
	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          jsonData,
	}, nil)

	if err != nil {
		return fmt.Errorf("failed to publish to Kafka: %w", err)
	}

	logger.Debugf("Published event to topic %s: %s", topic, event["eventType"])
	return nil
}

func extractDeckName(filePath string) string {
	base := filepath.Base(filePath)
	// Remove extensions
	name := base
	if idx := lastIndex(name, ".deck"); idx >= 0 {
		name = name[:idx]
	}
	if idx := lastIndex(name, ".txt"); idx >= 0 {
		name = name[:idx]
	}
	// Replace hyphens and underscores with spaces
	name = replaceAll(name, "-", " ")
	name = replaceAll(name, "_", " ")
	// Title case
	return titleCase(name)
}

// Helper functions to avoid additional imports
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for start < end && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func lastIndex(s, substr string) int {
	n := len(substr)
	for i := len(s) - n; i >= 0; i-- {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}

func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	result := ""
	start := 0
	for {
		idx := -1
		for i := start; i <= len(s)-len(old); i++ {
			if s[i:i+len(old)] == old {
				idx = i
				break
			}
		}
		if idx == -1 {
			return result + s[start:]
		}
		result += s[start:idx] + new
		start = idx + len(old)
	}
}

func titleCase(s string) string {
	result := ""
	wasSpace := true
	for _, r := range s {
		if r == ' ' {
			wasSpace = true
			result += " "
		} else if wasSpace {
			if r >= 'a' && r <= 'z' {
				result += string(r - 32)
			} else {
				result += string(r)
			}
			wasSpace = false
		} else {
			result += string(r)
		}
	}
	return result
}