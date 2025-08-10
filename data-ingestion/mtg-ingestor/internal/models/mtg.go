package models

import "time"

// Card represents an MTG card from MTGJSON
type Card struct {
	UUID            string                 `json:"uuid"`
	Name            string                 `json:"name"`
	ManaCost        string                 `json:"manaCost,omitempty"`
	ConvertedMana   float64                `json:"convertedManaCost"`
	Type            string                 `json:"type"`
	Text            string                 `json:"text,omitempty"`
	Power           string                 `json:"power,omitempty"`
	Toughness       string                 `json:"toughness,omitempty"`
	Colors          []string               `json:"colors,omitempty"`
	ColorIdentity   []string               `json:"colorIdentity,omitempty"`
	SetCode         string                 `json:"setCode"`
	Rarity          string                 `json:"rarity"`
	Artist          string                 `json:"artist,omitempty"`
	Number          string                 `json:"number"`
	Layout          string                 `json:"layout"`
	Prices          map[string]interface{} `json:"prices,omitempty"`
	Legalities      map[string]string      `json:"legalities,omitempty"`
	Subtypes        []string               `json:"subtypes,omitempty"`
	Supertypes      []string               `json:"supertypes,omitempty"`
	Types           []string               `json:"types,omitempty"`
	Keywords        []string               `json:"keywords,omitempty"`
	ProcessedAt     time.Time              `json:"processedAt"`
}

// Set represents an MTG set from MTGJSON
type Set struct {
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	ReleaseDate  string    `json:"releaseDate"`
	BaseSetSize  int       `json:"baseSetSize"`
	TotalSetSize int       `json:"totalSetSize"`
	Cards        []Card    `json:"cards"`
	ProcessedAt  time.Time `json:"processedAt"`
}

// KafkaEvent represents an event to be published to Kafka
type KafkaEvent struct {
	EventType   string      `json:"eventType"`
	EventID     string      `json:"eventId"`
	Timestamp   time.Time   `json:"timestamp"`
	Data        interface{} `json:"data"`
	Source      string      `json:"source"`
	Version     string      `json:"version"`
}

// CardEvent is a Kafka event for card data
type CardEvent struct {
	KafkaEvent
	Card Card `json:"card"`
}

// SetEvent is a Kafka event for set data
type SetEvent struct {
	KafkaEvent
	Set Set `json:"set"`
}