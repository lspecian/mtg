package com.mtg.flink;

import org.apache.flink.api.common.eventtime.WatermarkStrategy;
import org.apache.flink.api.common.functions.MapFunction;
import org.apache.flink.api.common.functions.ReduceFunction;
import org.apache.flink.api.common.serialization.SimpleStringSchema;
import org.apache.flink.api.java.tuple.Tuple2;
import org.apache.flink.api.java.tuple.Tuple3;
import org.apache.flink.connector.kafka.sink.KafkaRecordSerializationSchema;
import org.apache.flink.connector.kafka.sink.KafkaSink;
import org.apache.flink.connector.kafka.source.KafkaSource;
import org.apache.flink.connector.kafka.source.enumerator.initializer.OffsetsInitializer;
import org.apache.flink.streaming.api.datastream.DataStream;
import org.apache.flink.streaming.api.datastream.KeyedStream;
import org.apache.flink.streaming.api.environment.StreamExecutionEnvironment;
import org.apache.flink.streaming.api.functions.co.CoFlatMapFunction;
import org.apache.flink.streaming.api.windowing.assigners.TumblingProcessingTimeWindows;
import org.apache.flink.streaming.api.windowing.time.Time;
import org.apache.flink.util.Collector;
import org.apache.kafka.clients.consumer.OffsetResetStrategy;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.time.Duration;
import java.util.HashMap;
import java.util.Map;
import java.util.Properties;

public class DeckValueProcessor {
    
    private static final ObjectMapper mapper = new ObjectMapper();
    
    public static void main(String[] args) throws Exception {
        // Set up the execution environment
        StreamExecutionEnvironment env = StreamExecutionEnvironment.getExecutionEnvironment();
        env.setParallelism(1);
        
        String kafkaBootstrapServers = System.getenv("KAFKA_BROKERS");
        if (kafkaBootstrapServers == null) {
            kafkaBootstrapServers = "kafka:29092";
        }
        
        // Create Kafka source for deck-cards topic
        KafkaSource<String> deckCardsSource = KafkaSource.<String>builder()
            .setBootstrapServers(kafkaBootstrapServers)
            .setTopics("mtg.deck-cards")
            .setGroupId("deck-value-processor")
            .setStartingOffsets(OffsetsInitializer.committedOffsets(OffsetResetStrategy.EARLIEST))
            .setValueOnlyDeserializer(new SimpleStringSchema())
            .build();
        
        // Create Kafka source for prices topic
        KafkaSource<String> pricesSource = KafkaSource.<String>builder()
            .setBootstrapServers(kafkaBootstrapServers)
            .setTopics("mtg.prices")
            .setGroupId("deck-value-processor-prices")
            .setStartingOffsets(OffsetsInitializer.latest())
            .setValueOnlyDeserializer(new SimpleStringSchema())
            .build();
        
        // Create data streams
        DataStream<String> deckCardsStream = env.fromSource(
            deckCardsSource,
            WatermarkStrategy.forBoundedOutOfOrderness(Duration.ofSeconds(5)),
            "Deck Cards Source"
        );
        
        DataStream<String> pricesStream = env.fromSource(
            pricesSource,
            WatermarkStrategy.forBoundedOutOfOrderness(Duration.ofSeconds(5)),
            "Prices Source"
        );
        
        // Parse deck card events
        DataStream<DeckCard> deckCards = deckCardsStream
            .map(new MapFunction<String, DeckCard>() {
                @Override
                public DeckCard map(String value) throws Exception {
                    JsonNode json = mapper.readTree(value);
                    JsonNode data = json.get("data");
                    
                    return new DeckCard(
                        data.get("deck_id").asText(),
                        data.get("deck_name").asText(),
                        data.get("card_name").asText(),
                        data.get("quantity").asInt()
                    );
                }
            });
        
        // Parse price events and create a price map
        DataStream<CardPrice> prices = pricesStream
            .map(new MapFunction<String, CardPrice>() {
                @Override
                public CardPrice map(String value) throws Exception {
                    JsonNode json = mapper.readTree(value);
                    JsonNode data = json.get("data");
                    
                    // Extract card name from the price data
                    // We'll need to match by card name
                    String cardUuid = data.get("card_uuid").asText();
                    double price = data.has("price") ? data.get("price").asDouble() : 0.0;
                    
                    return new CardPrice(cardUuid, price);
                }
            });
        
        // Connect streams and calculate deck values
        DataStream<DeckValue> deckValues = deckCards
            .keyBy(card -> card.deckId)
            .window(TumblingProcessingTimeWindows.of(Time.seconds(10)))
            .reduce(new ReduceFunction<DeckCard>() {
                @Override
                public DeckCard reduce(DeckCard card1, DeckCard card2) {
                    // Aggregate cards by deck
                    return new DeckCard(
                        card1.deckId,
                        card1.deckName,
                        card1.cardName + "," + card2.cardName,
                        card1.quantity + card2.quantity
                    );
                }
            })
            .map(new MapFunction<DeckCard, DeckValue>() {
                @Override
                public DeckValue map(DeckCard aggregated) throws Exception {
                    // For now, use estimated prices
                    // In production, this would join with actual price data
                    double estimatedValue = aggregated.quantity * 5.0; // $5 average per card
                    
                    return new DeckValue(
                        aggregated.deckId,
                        aggregated.deckName,
                        aggregated.quantity,
                        estimatedValue,
                        System.currentTimeMillis()
                    );
                }
            });
        
        // Convert to JSON for output
        DataStream<String> deckValueJson = deckValues
            .map(new MapFunction<DeckValue, String>() {
                @Override
                public String map(DeckValue value) throws Exception {
                    ObjectNode event = mapper.createObjectNode();
                    event.put("eventType", "deck.value.calculated");
                    event.put("eventId", java.util.UUID.randomUUID().toString());
                    event.put("timestamp", System.currentTimeMillis());
                    event.put("source", "flink-deck-processor");
                    event.put("version", "v1");
                    
                    ObjectNode data = mapper.createObjectNode();
                    data.put("deck_id", value.deckId);
                    data.put("deck_name", value.deckName);
                    data.put("total_cards", value.totalCards);
                    data.put("total_value", value.totalValue);
                    data.put("calculated_at", value.timestamp);
                    
                    event.set("data", data);
                    return mapper.writeValueAsString(event);
                }
            });
        
        // Create Kafka sink for deck values
        KafkaSink<String> deckValueSink = KafkaSink.<String>builder()
            .setBootstrapServers(kafkaBootstrapServers)
            .setRecordSerializer(
                KafkaRecordSerializationSchema.builder()
                    .setTopic("mtg.deck-values")
                    .setValueSerializationSchema(new SimpleStringSchema())
                    .build()
            )
            .build();
        
        // Write to Kafka
        deckValueJson.sinkTo(deckValueSink);
        
        // Execute the job
        env.execute("MTG Deck Value Processor");
    }
    
    // Data classes
    public static class DeckCard {
        public String deckId;
        public String deckName;
        public String cardName;
        public int quantity;
        
        public DeckCard() {}
        
        public DeckCard(String deckId, String deckName, String cardName, int quantity) {
            this.deckId = deckId;
            this.deckName = deckName;
            this.cardName = cardName;
            this.quantity = quantity;
        }
    }
    
    public static class CardPrice {
        public String cardId;
        public double price;
        
        public CardPrice() {}
        
        public CardPrice(String cardId, double price) {
            this.cardId = cardId;
            this.price = price;
        }
    }
    
    public static class DeckValue {
        public String deckId;
        public String deckName;
        public int totalCards;
        public double totalValue;
        public long timestamp;
        
        public DeckValue() {}
        
        public DeckValue(String deckId, String deckName, int totalCards, double totalValue, long timestamp) {
            this.deckId = deckId;
            this.deckName = deckName;
            this.totalCards = totalCards;
            this.totalValue = totalValue;
            this.timestamp = timestamp;
        }
    }
}