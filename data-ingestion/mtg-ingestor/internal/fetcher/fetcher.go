package fetcher

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mtg/mtg-ingestor/internal/models"
	"github.com/sirupsen/logrus"
)

type MTGFetcher struct {
	logger  *logrus.Logger
	client  *http.Client
	baseURL string
}

func NewMTGFetcher(logger *logrus.Logger) *MTGFetcher {
	return &MTGFetcher{
		logger:  logger,
		client:  &http.Client{Timeout: 30 * time.Minute},
		baseURL: "https://mtgjson.com/api/v5",
	}
}

// FetchAllSets fetches all MTG sets data
func (f *MTGFetcher) FetchAllSets() (map[string]models.Set, error) {
	url := fmt.Sprintf("%s/AllSets.json.gz", f.baseURL)
	f.logger.Infof("Fetching MTG data from %s", url)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Decompress gzip
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Read and parse JSON
	data, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var allSets map[string]models.Set
	if err := json.Unmarshal(data, &allSets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Add processed timestamp to each set
	now := time.Now()
	for code, set := range allSets {
		set.ProcessedAt = now
		// Add processed timestamp to each card
		for i := range set.Cards {
			set.Cards[i].ProcessedAt = now
		}
		allSets[code] = set
	}

	f.logger.Infof("Successfully fetched %d sets", len(allSets))
	return allSets, nil
}

// FetchAtomicCards fetches individual card data
func (f *MTGFetcher) FetchAtomicCards() (map[string]models.Card, error) {
	url := fmt.Sprintf("%s/AtomicCards.json.gz", f.baseURL)
	f.logger.Infof("Fetching atomic cards from %s", url)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch atomic cards: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	data, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// AtomicCards has structure: {"meta": {}, "data": {"cardName": [cardVariants]}}
	var atomicResponse struct {
		Meta interface{}                `json:"meta"`
		Data map[string][]interface{} `json:"data"`
	}
	
	if err := json.Unmarshal(data, &atomicResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal atomic cards: %w", err)
	}

	// Process each card and its variants
	cards := make(map[string]models.Card)
	now := time.Now()
	cardCount := 0
	
	for cardName, variants := range atomicResponse.Data {
		// Take the first variant as the canonical version
		if len(variants) > 0 {
			cardBytes, err := json.Marshal(variants[0])
			if err != nil {
				f.logger.Debugf("Failed to marshal card %s: %v", cardName, err)
				continue
			}
			
			var card models.Card
			if err := json.Unmarshal(cardBytes, &card); err != nil {
				f.logger.Debugf("Failed to unmarshal card %s: %v", cardName, err)
				continue
			}
			
			// Ensure we have a name
			if card.Name == "" {
				card.Name = cardName
			}
			
			// Generate a UUID if not present
			if card.UUID == "" {
				card.UUID = fmt.Sprintf("%s_%d", cardName, cardCount)
			}
			
			card.ProcessedAt = now
			cards[card.UUID] = card
			cardCount++
		}
	}

	f.logger.Infof("Successfully fetched %d unique cards", len(cards))
	return cards, nil
}

// PriceData represents individual price data for a card
type PriceData struct {
	CardUUID     string    `json:"card_uuid"`
	Format       string    `json:"format"`      // paper, mtgo
	Source       string    `json:"source"`      // cardkingdom, tcgplayer, etc
	Type         string    `json:"type"`        // retail, buylist
	Foil         bool      `json:"foil"`
	Date         string    `json:"date"`
	Price        float64   `json:"price"`
}

// FetchPrices fetches price data and returns individual price records
func (f *MTGFetcher) FetchPrices() ([]PriceData, error) {
	url := fmt.Sprintf("%s/AllPrices.json.gz", f.baseURL)
	f.logger.Infof("Fetching price data from %s", url)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	data, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the structure: {"meta": {}, "data": {cardUUID: {format: {source: {type: {foilStatus: {date: price}}}}}}}
	var priceResponse struct {
		Meta interface{}            `json:"meta"`
		Data map[string]interface{} `json:"data"`
	}
	
	if err := json.Unmarshal(data, &priceResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prices: %w", err)
	}

	// Flatten price data into individual records
	var prices []PriceData
	for cardUUID, formatData := range priceResponse.Data {
		if formatMap, ok := formatData.(map[string]interface{}); ok {
			for format, sourceData := range formatMap {
				if sourceMap, ok := sourceData.(map[string]interface{}); ok {
					for source, typeData := range sourceMap {
						if typeMap, ok := typeData.(map[string]interface{}); ok {
							for priceType, foilData := range typeMap {
								if foilMap, ok := foilData.(map[string]interface{}); ok {
									for foilStatus, dateData := range foilMap {
										isFoil := foilStatus == "foil"
										if dateMap, ok := dateData.(map[string]interface{}); ok {
											for date, price := range dateMap {
												if priceFloat, ok := price.(float64); ok {
													prices = append(prices, PriceData{
														CardUUID: cardUUID,
														Format:   format,
														Source:   source,
														Type:     priceType,
														Foil:     isFoil,
														Date:     date,
														Price:    priceFloat,
													})
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	f.logger.Infof("Successfully fetched %d price records", len(prices))
	return prices, nil
}