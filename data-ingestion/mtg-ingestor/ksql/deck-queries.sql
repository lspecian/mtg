-- KSQL Queries for Deck Analysis

-- 1. Create stream for deck values
CREATE STREAM deck_values_stream (
  eventType VARCHAR,
  eventId VARCHAR,
  timestamp BIGINT,
  source VARCHAR,
  version VARCHAR,
  data STRUCT<
    deck_id VARCHAR,
    deck_name VARCHAR,
    total_cards INT,
    total_value DOUBLE,
    calculated_at BIGINT
  >
) WITH (
  KAFKA_TOPIC='mtg.deck-values',
  VALUE_FORMAT='JSON'
);

-- 2. Create stream for deck information
CREATE STREAM decks_stream (
  eventType VARCHAR,
  eventId VARCHAR,
  timestamp BIGINT,
  source VARCHAR,
  version VARCHAR,
  data STRUCT<
    id VARCHAR,
    name VARCHAR,
    file_path VARCHAR,
    total_cards INT,
    unique_cards INT,
    ingested_at VARCHAR
  >
) WITH (
  KAFKA_TOPIC='mtg.decks',
  VALUE_FORMAT='JSON'
);

-- 3. Query to list all deck values (real-time)
SELECT 
  data->deck_name AS deck_name,
  data->total_cards AS cards,
  data->total_value AS value,
  TIMESTAMPTOSTRING(data->calculated_at, 'yyyy-MM-dd HH:mm:ss') AS calculated_time
FROM deck_values_stream
EMIT CHANGES;

-- 4. Create a table with latest deck values
CREATE TABLE deck_values_latest AS
SELECT 
  data->deck_id AS deck_id,
  LATEST_BY_OFFSET(data->deck_name) AS deck_name,
  LATEST_BY_OFFSET(data->total_cards) AS total_cards,
  LATEST_BY_OFFSET(data->total_value) AS total_value,
  LATEST_BY_OFFSET(data->calculated_at) AS last_updated
FROM deck_values_stream
GROUP BY data->deck_id
EMIT CHANGES;

-- 5. Query deck values sorted by value (highest first)
SELECT 
  deck_name,
  total_cards,
  total_value,
  CAST(total_value / total_cards AS DOUBLE) AS avg_card_value
FROM deck_values_latest
WHERE total_value > 0
EMIT CHANGES;

-- 6. Show deck statistics
SELECT
  COUNT(*) AS total_decks,
  SUM(total_value) AS combined_value,
  AVG(total_value) AS average_deck_value,
  MAX(total_value) AS most_expensive_deck,
  MIN(total_value) AS cheapest_deck
FROM deck_values_latest
EMIT CHANGES;

-- 7. Join decks with their values for complete information
CREATE STREAM enriched_decks AS
SELECT 
  d.data->name AS deck_name,
  d.data->unique_cards AS unique_cards,
  d.data->total_cards AS total_cards,
  v.data->total_value AS total_value,
  CAST(v.data->total_value / d.data->total_cards AS DOUBLE) AS avg_card_value,
  d.data->file_path AS file_path
FROM decks_stream d
INNER JOIN deck_values_stream v 
  WITHIN 1 HOUR
  ON d.data->id = v.data->deck_id
EMIT CHANGES;

-- 8. Query to show all decks with values
SELECT * FROM enriched_decks EMIT CHANGES;

-- 9. Find most expensive decks (over $1000)
SELECT 
  deck_name,
  total_value,
  total_cards
FROM deck_values_latest
WHERE total_value > 1000
EMIT CHANGES;

-- 10. Compare deck values by card count
SELECT 
  CASE 
    WHEN total_cards <= 60 THEN 'Standard (60 cards)'
    WHEN total_cards <= 75 THEN 'Limited (75 cards)'
    WHEN total_cards = 100 THEN 'Commander (100 cards)'
    ELSE 'Other'
  END AS format,
  COUNT(*) AS deck_count,
  AVG(total_value) AS avg_value
FROM deck_values_latest
GROUP BY 
  CASE 
    WHEN total_cards <= 60 THEN 'Standard (60 cards)'
    WHEN total_cards <= 75 THEN 'Limited (75 cards)'
    WHEN total_cards = 100 THEN 'Commander (100 cards)'
    ELSE 'Other'
  END
EMIT CHANGES;