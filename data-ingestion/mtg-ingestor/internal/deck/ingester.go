package deck

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// DeckCard represents a card in a deck
type DeckCard struct {
	Quantity int    `json:"quantity"`
	Name     string `json:"name"`
}

// Deck represents a complete deck
type Deck struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	FilePath    string      `json:"file_path"`
	Cards       []DeckCard  `json:"cards"`
	TotalCards  int         `json:"total_cards"`
	UniqueCards int         `json:"unique_cards"`
	IngestedAt  time.Time   `json:"ingested_at"`
}

// DeckEvent represents a deck event for Kafka
type DeckEvent struct {
	EventType string      `json:"eventType"`
	EventID   string      `json:"eventId"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
	Version   string      `json:"version"`
	Data      interface{} `json:"data"`
}

// Ingester handles deck file ingestion
type Ingester struct {
	logger *logrus.Logger
}

// NewIngester creates a new deck ingester
func NewIngester(logger *logrus.Logger) *Ingester {
	return &Ingester{
		logger: logger,
	}
}

// IngestDirectory processes all deck files in a directory
func (i *Ingester) IngestDirectory(dirPath string) ([]Deck, error) {
	var decks []Deck

	files, err := filepath.Glob(filepath.Join(dirPath, "*.deck"))
	if err != nil {
		return nil, fmt.Errorf("failed to list deck files: %w", err)
	}

	// Also check for .txt deck files
	txtFiles, err := filepath.Glob(filepath.Join(dirPath, "*.deck.txt"))
	if err == nil {
		files = append(files, txtFiles...)
	}

	i.logger.Infof("Found %d deck files to process", len(files))

	for _, filePath := range files {
		deck, err := i.IngestFile(filePath)
		if err != nil {
			i.logger.WithError(err).Errorf("Failed to ingest deck file: %s", filePath)
			continue
		}
		decks = append(decks, *deck)
	}

	return decks, nil
}

// IngestFile processes a single deck file
func (i *Ingester) IngestFile(filePath string) (*Deck, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	deck := &Deck{
		ID:         uuid.New().String(),
		Name:       extractDeckName(filePath),
		FilePath:   filePath,
		Cards:      []DeckCard{},
		IngestedAt: time.Now(),
	}

	scanner := bufio.NewScanner(file)
	cardRegex := regexp.MustCompile(`^(\d+)\s+(.+)$`)
	totalCards := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}

		matches := cardRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			quantity, err := strconv.Atoi(matches[1])
			if err != nil {
				i.logger.Warnf("Invalid quantity in line: %s", line)
				continue
			}

			cardName := strings.TrimSpace(matches[2])
			deck.Cards = append(deck.Cards, DeckCard{
				Quantity: quantity,
				Name:     cardName,
			})
			totalCards += quantity
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	deck.TotalCards = totalCards
	deck.UniqueCards = len(deck.Cards)

	i.logger.Infof("Ingested deck '%s': %d unique cards, %d total cards", 
		deck.Name, deck.UniqueCards, deck.TotalCards)

	return deck, nil
}

// CreateDeckEvent creates a Kafka event for a deck
func (i *Ingester) CreateDeckEvent(deck *Deck) DeckEvent {
	return DeckEvent{
		EventType: "deck.ingested",
		EventID:   uuid.New().String(),
		Timestamp: time.Now(),
		Source:    "deck-ingester",
		Version:   "v1",
		Data:      deck,
	}
}

// CreateDeckCardEvents creates individual card events for deck analysis
func (i *Ingester) CreateDeckCardEvents(deck *Deck) []DeckEvent {
	var events []DeckEvent

	for _, card := range deck.Cards {
		event := DeckEvent{
			EventType: "deck.card",
			EventID:   uuid.New().String(),
			Timestamp: time.Now(),
			Source:    "deck-ingester",
			Version:   "v1",
			Data: map[string]interface{}{
				"deck_id":   deck.ID,
				"deck_name": deck.Name,
				"card_name": card.Name,
				"quantity":  card.Quantity,
			},
		}
		events = append(events, event)
	}

	return events
}

// extractDeckName extracts deck name from file path
func extractDeckName(filePath string) string {
	base := filepath.Base(filePath)
	// Remove extensions
	name := strings.TrimSuffix(base, ".deck")
	name = strings.TrimSuffix(name, ".txt")
	// Replace hyphens and underscores with spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	// Title case
	return strings.Title(name)
}

// ToJSON converts deck to JSON
func (d *Deck) ToJSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}