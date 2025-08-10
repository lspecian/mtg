package kafka

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	"github.com/mtg/mtg-ingestor/internal/models"
	"github.com/sirupsen/logrus"
)

type Producer struct {
	producer *kafka.Producer
	logger   *logrus.Logger
	topics   map[string]string
}

type ProducerConfig struct {
	Brokers       string
	CardsTopic    string
	SetsTopic     string
	PricesTopic   string
	Logger        *logrus.Logger
}

func NewProducer(config ProducerConfig) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  config.Brokers,
		"client.id":         "mtg-ingestor",
		"acks":             "all",
		"retries":          10,
		"retry.backoff.ms": 100,
		"compression.type": "snappy",
		"linger.ms":       10,
		"batch.size":      16384,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	producer := &Producer{
		producer: p,
		logger:   config.Logger,
		topics: map[string]string{
			"cards":  config.CardsTopic,
			"sets":   config.SetsTopic,
			"prices": config.PricesTopic,
		},
	}

	// Start delivery report handler
	go producer.handleDeliveryReports()

	return producer, nil
}

func (p *Producer) handleDeliveryReports() {
	for e := range p.producer.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				p.logger.Errorf("Delivery failed: %v", ev.TopicPartition.Error)
			} else {
				p.logger.Debugf("Delivered message to %v", ev.TopicPartition)
			}
		}
	}
}

// PublishCard publishes a card event to Kafka
func (p *Producer) PublishCard(card models.Card) error {
	event := models.CardEvent{
		KafkaEvent: models.KafkaEvent{
			EventType: "card.created",
			EventID:   uuid.New().String(),
			Timestamp: time.Now(),
			Source:    "mtgjson",
			Version:   "v5",
		},
		Card: card,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal card event: %w", err)
	}

	topic := p.topics["cards"]
	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(card.UUID),
		Value:          data,
		Headers: []kafka.Header{
			{Key: "eventType", Value: []byte("card.created")},
			{Key: "source", Value: []byte("mtgjson")},
		},
	}, nil)

	if err != nil {
		return fmt.Errorf("failed to produce card message: %w", err)
	}

	return nil
}

// PublishSet publishes a set event to Kafka
func (p *Producer) PublishSet(set models.Set) error {
	// Create set event without cards (cards are published separately)
	setCopy := set
	setCopy.Cards = nil

	event := models.SetEvent{
		KafkaEvent: models.KafkaEvent{
			EventType: "set.created",
			EventID:   uuid.New().String(),
			Timestamp: time.Now(),
			Source:    "mtgjson",
			Version:   "v5",
		},
		Set: setCopy,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal set event: %w", err)
	}

	topic := p.topics["sets"]
	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(set.Code),
		Value:          data,
		Headers: []kafka.Header{
			{Key: "eventType", Value: []byte("set.created")},
			{Key: "source", Value: []byte("mtgjson")},
		},
	}, nil)

	if err != nil {
		return fmt.Errorf("failed to produce set message: %w", err)
	}

	// Publish each card in the set
	for _, card := range set.Cards {
		if err := p.PublishCard(card); err != nil {
			p.logger.Errorf("Failed to publish card %s: %v", card.Name, err)
		}
	}

	return nil
}

// PublishPrice publishes individual price data to Kafka
func (p *Producer) PublishPrice(price interface{}) error {
	event := map[string]interface{}{
		"eventType": "price.updated",
		"eventId":   uuid.New().String(),
		"timestamp": time.Now(),
		"source":    "mtgjson",
		"version":   "v5",
		"data":      price,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal price event: %w", err)
	}

	topic := p.topics["prices"]
	
	// Extract card UUID for key if possible
	key := fmt.Sprintf("price-%s", time.Now().Format("2006-01-02-15:04:05"))
	if priceMap, ok := price.(map[string]interface{}); ok {
		if uuid, ok := priceMap["card_uuid"].(string); ok {
			key = fmt.Sprintf("price-%s-%s", uuid, time.Now().Format("2006-01-02"))
		}
	}
	
	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          data,
		Headers: []kafka.Header{
			{Key: "eventType", Value: []byte("price.updated")},
			{Key: "source", Value: []byte("mtgjson")},
		},
	}, nil)

	if err != nil {
		return fmt.Errorf("failed to produce price message: %w", err)
	}

	return nil
}

// Flush waits for all messages to be delivered
func (p *Producer) Flush(timeoutMs int) int {
	return p.producer.Flush(timeoutMs)
}

// Close closes the producer
func (p *Producer) Close() {
	p.producer.Close()
}