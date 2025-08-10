package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/mtg/mtg-ingestor/internal/deck"
	"github.com/sirupsen/logrus"
)

func main() {
	dirPath := flag.String("dir", "../../decks", "Directory containing deck files")
	flag.Parse()

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	ingester := deck.NewIngester(logger)

	fmt.Printf("Ingesting decks from: %s\n\n", *dirPath)
	decks, err := ingester.IngestDirectory(*dirPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Successfully ingested %d decks:\n\n", len(decks))

	for _, d := range decks {
		fmt.Printf("Deck: %s\n", d.Name)
		fmt.Printf("  File: %s\n", d.FilePath)
		fmt.Printf("  Total Cards: %d\n", d.TotalCards)
		fmt.Printf("  Unique Cards: %d\n", d.UniqueCards)
		fmt.Printf("  Sample Cards:\n")
		
		maxCards := 5
		if len(d.Cards) < maxCards {
			maxCards = len(d.Cards)
		}
		
		for i := 0; i < maxCards; i++ {
			card := d.Cards[i]
			fmt.Printf("    - %dx %s\n", card.Quantity, card.Name)
		}
		
		if len(d.Cards) > 5 {
			fmt.Printf("    ... and %d more cards\n", len(d.Cards)-5)
		}
		
		fmt.Println()
	}

	// Test event creation
	if len(decks) > 0 {
		fmt.Println("Sample Kafka Events:")
		fmt.Println("====================")
		
		firstDeck := &decks[0]
		event := ingester.CreateDeckEvent(firstDeck)
		
		eventJSON, _ := json.MarshalIndent(event, "", "  ")
		fmt.Printf("Deck Event:\n%s\n\n", string(eventJSON))
		
		cardEvents := ingester.CreateDeckCardEvents(firstDeck)
		if len(cardEvents) > 0 {
			firstCardEvent, _ := json.MarshalIndent(cardEvents[0], "", "  ")
			fmt.Printf("Sample Card Event:\n%s\n", string(firstCardEvent))
			fmt.Printf("(Total %d card events would be created)\n", len(cardEvents))
		}
	}
}