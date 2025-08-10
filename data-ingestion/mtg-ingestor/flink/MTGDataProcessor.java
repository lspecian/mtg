package com.mtg.flink;

import org.apache.flink.api.common.eventtime.WatermarkStrategy;
import org.apache.flink.api.common.serialization.SimpleStringSchema;
import org.apache.flink.connector.base.DeliveryGuarantee;
import org.apache.flink.connector.jdbc.JdbcConnectionOptions;
import org.apache.flink.connector.jdbc.JdbcExecutionOptions;
import org.apache.flink.connector.jdbc.JdbcSink;
import org.apache.flink.connector.kafka.sink.KafkaRecordSerializationSchema;
import org.apache.flink.connector.kafka.sink.KafkaSink;
import org.apache.flink.connector.kafka.source.KafkaSource;
import org.apache.flink.connector.kafka.source.enumerator.initializer.OffsetsInitializer;
import org.apache.flink.formats.json.JsonDeserializationSchema;
import org.apache.flink.streaming.api.datastream.DataStream;
import org.apache.flink.streaming.api.environment.StreamExecutionEnvironment;
import org.apache.flink.streaming.api.functions.ProcessFunction;
import org.apache.flink.streaming.api.functions.sink.filesystem.StreamingFileSink;
import org.apache.flink.streaming.api.functions.sink.filesystem.rollingpolicies.DefaultRollingPolicy;
import org.apache.flink.util.Collector;
import org.apache.flink.core.fs.Path;
import org.apache.flink.formats.parquet.avro.ParquetAvroWriters;

import java.time.Duration;
import java.util.Properties;

public class MTGDataProcessor {
    
    public static void main(String[] args) throws Exception {
        // Set up execution environment
        StreamExecutionEnvironment env = StreamExecutionEnvironment.getExecutionEnvironment();
        env.enableCheckpointing(60000); // Checkpoint every 60 seconds
        
        // Kafka source configuration
        KafkaSource<Card> cardSource = KafkaSource.<Card>builder()
            .setBootstrapServers("kafka-broker-1:9092,kafka-broker-2:9092,kafka-broker-3:9092")
            .setTopics("mtg.cards")
            .setGroupId("mtg-flink-processor")
            .setStartingOffsets(OffsetsInitializer.latest())
            .setValueOnlyDeserializer(new JsonDeserializationSchema<>(Card.class))
            .build();
        
        // Create data stream from Kafka
        DataStream<Card> cardStream = env.fromSource(
            cardSource,
            WatermarkStrategy.<Card>forBoundedOutOfOrderness(Duration.ofSeconds(20)),
            "Card Source"
        );
        
        // Process and enrich card data
        DataStream<EnrichedCard> enrichedCards = cardStream
            .process(new CardEnrichmentFunction())
            .name("Card Enrichment");
        
        // Filter rare and mythic cards
        DataStream<EnrichedCard> rareCards = enrichedCards
            .filter(card -> "rare".equals(card.getRarity()) || "mythic".equals(card.getRarity()))
            .name("Filter Rare Cards");
        
        // Write to S3 in Parquet format
        StreamingFileSink<EnrichedCard> s3Sink = StreamingFileSink
            .forBulkFormat(
                new Path("s3://mtg-data/processed/cards/"),
                ParquetAvroWriters.forReflectRecord(EnrichedCard.class)
            )
            .withRollingPolicy(
                DefaultRollingPolicy.builder()
                    .withRolloverInterval(Duration.ofMinutes(15))
                    .withInactivityInterval(Duration.ofMinutes(5))
                    .withMaxPartSize(1024 * 1024 * 128) // 128 MB
                    .build()
            )
            .build();
        
        enrichedCards.addSink(s3Sink).name("S3 Sink");
        
        // Write to PostgreSQL
        enrichedCards.addSink(
            JdbcSink.sink(
                "INSERT INTO cards (uuid, name, mana_cost, type, rarity, set_code, processed_at) " +
                "VALUES (?, ?, ?, ?, ?, ?, ?) " +
                "ON CONFLICT (uuid) DO UPDATE SET " +
                "name = EXCLUDED.name, " +
                "mana_cost = EXCLUDED.mana_cost, " +
                "type = EXCLUDED.type, " +
                "rarity = EXCLUDED.rarity, " +
                "set_code = EXCLUDED.set_code, " +
                "processed_at = EXCLUDED.processed_at",
                (statement, card) -> {
                    statement.setString(1, card.getUuid());
                    statement.setString(2, card.getName());
                    statement.setString(3, card.getManaCost());
                    statement.setString(4, card.getType());
                    statement.setString(5, card.getRarity());
                    statement.setString(6, card.getSetCode());
                    statement.setTimestamp(7, new java.sql.Timestamp(card.getProcessedAt()));
                },
                JdbcExecutionOptions.builder()
                    .withBatchSize(1000)
                    .withBatchIntervalMs(200)
                    .withMaxRetries(5)
                    .build(),
                new JdbcConnectionOptions.JdbcConnectionOptionsBuilder()
                    .withUrl("jdbc:postgresql://postgres:5432/mtg")
                    .withDriverName("org.postgresql.Driver")
                    .withUsername("mtg_user")
                    .withPassword(System.getenv("POSTGRES_PASSWORD"))
                    .build()
            )
        ).name("PostgreSQL Sink");
        
        // Calculate statistics and write to separate Kafka topic
        DataStream<CardStatistics> statistics = enrichedCards
            .keyBy(card -> card.getSetCode())
            .window(TumblingProcessingTimeWindows.of(Time.minutes(5)))
            .aggregate(new CardStatisticsAggregator())
            .name("Calculate Statistics");
        
        KafkaSink<CardStatistics> statsSink = KafkaSink.<CardStatistics>builder()
            .setBootstrapServers("kafka-broker-1:9092,kafka-broker-2:9092,kafka-broker-3:9092")
            .setRecordSerializer(KafkaRecordSerializationSchema.builder()
                .setTopic("mtg.statistics")
                .setValueSerializationSchema(new JsonSerializationSchema<>())
                .build()
            )
            .setDeliveryGuarantee(DeliveryGuarantee.AT_LEAST_ONCE)
            .build();
        
        statistics.sinkTo(statsSink).name("Statistics Kafka Sink");
        
        // Execute the job
        env.execute("MTG Data Processing Pipeline");
    }
    
    // Custom function to enrich card data
    public static class CardEnrichmentFunction extends ProcessFunction<Card, EnrichedCard> {
        @Override
        public void processElement(Card card, Context ctx, Collector<EnrichedCard> out) {
            EnrichedCard enriched = new EnrichedCard();
            enriched.setUuid(card.getUuid());
            enriched.setName(card.getName());
            enriched.setManaCost(card.getManaCost());
            enriched.setType(card.getType());
            enriched.setRarity(card.getRarity());
            enriched.setSetCode(card.getSetCode());
            enriched.setProcessedAt(System.currentTimeMillis());
            
            // Calculate color distribution
            if (card.getColors() != null) {
                enriched.setColorCount(card.getColors().size());
                enriched.setMulticolored(card.getColors().size() > 1);
            }
            
            // Calculate converted mana cost tier
            if (card.getConvertedManaCost() <= 2) {
                enriched.setManaCostTier("low");
            } else if (card.getConvertedManaCost() <= 4) {
                enriched.setManaCostTier("medium");
            } else {
                enriched.setManaCostTier("high");
            }
            
            out.collect(enriched);
        }
    }
}