-- KSQL Queries for MTG Data Processing and Analysis

-- Create streams from Kafka topics
CREATE STREAM cards_stream (
  eventType VARCHAR,
  eventId VARCHAR,
  timestamp BIGINT,
  source VARCHAR,
  version VARCHAR,
  card STRUCT<
    uuid VARCHAR,
    name VARCHAR,
    manaCost VARCHAR,
    convertedManaCost DOUBLE,
    type VARCHAR,
    text VARCHAR,
    power VARCHAR,
    toughness VARCHAR,
    colors ARRAY<VARCHAR>,
    colorIdentity ARRAY<VARCHAR>,
    setCode VARCHAR,
    rarity VARCHAR,
    artist VARCHAR,
    number VARCHAR,
    layout VARCHAR
  >
) WITH (
  KAFKA_TOPIC='mtg.cards',
  VALUE_FORMAT='JSON'
);

CREATE STREAM prices_stream (
  eventType VARCHAR,
  eventId VARCHAR,
  timestamp BIGINT,
  source VARCHAR,
  version VARCHAR,
  data STRUCT<
    card_uuid VARCHAR,
    format VARCHAR,
    source VARCHAR,
    type VARCHAR,
    foil BOOLEAN,
    date VARCHAR,
    price DOUBLE
  >
) WITH (
  KAFKA_TOPIC='mtg.prices',
  VALUE_FORMAT='JSON'
);

CREATE STREAM sets_stream (
  eventType VARCHAR,
  eventId VARCHAR,
  timestamp BIGINT,
  source VARCHAR,
  version VARCHAR,
  set STRUCT<
    code VARCHAR,
    name VARCHAR,
    type VARCHAR,
    releaseDate VARCHAR,
    baseSetSize INT,
    totalSetSize INT
  >
) WITH (
  KAFKA_TOPIC='mtg.sets',
  VALUE_FORMAT='JSON'
);

-- Create a table for card lookup
CREATE TABLE cards_table AS
  SELECT 
    card->uuid AS card_uuid,
    LATEST_BY_OFFSET(card->name) AS name,
    LATEST_BY_OFFSET(card->manaCost) AS mana_cost,
    LATEST_BY_OFFSET(card->convertedManaCost) AS cmc,
    LATEST_BY_OFFSET(card->type) AS type,
    LATEST_BY_OFFSET(card->rarity) AS rarity,
    LATEST_BY_OFFSET(card->setCode) AS set_code
  FROM cards_stream
  GROUP BY card->uuid
  EMIT CHANGES;

-- Create enriched price stream with card names
CREATE STREAM enriched_prices AS
  SELECT 
    p.data->card_uuid AS card_uuid,
    c.name AS card_name,
    p.data->format AS format,
    p.data->source AS price_source,
    p.data->type AS price_type,
    p.data->foil AS is_foil,
    p.data->date AS price_date,
    p.data->price AS price,
    c.rarity AS rarity,
    c.type AS card_type
  FROM prices_stream p
  LEFT JOIN cards_table c ON p.data->card_uuid = c.card_uuid
  EMIT CHANGES;

-- Query: Find most expensive cards
CREATE TABLE expensive_cards AS
  SELECT
    card_uuid,
    card_name,
    MAX(price) AS max_price,
    COUNT(*) AS price_points
  FROM enriched_prices
  WHERE price > 100
  GROUP BY card_uuid, card_name
  EMIT CHANGES;

-- Query: Cards by rarity distribution
CREATE TABLE rarity_distribution AS
  SELECT
    card->rarity AS rarity,
    COUNT(*) AS card_count
  FROM cards_stream
  GROUP BY card->rarity
  EMIT CHANGES;

-- Query: Price trends by date
CREATE TABLE daily_price_stats AS
  SELECT
    data->date AS price_date,
    COUNT(*) AS total_prices,
    AVG(data->price) AS avg_price,
    MAX(data->price) AS max_price,
    MIN(data->price) AS min_price
  FROM prices_stream
  WHERE data->price > 0
  GROUP BY data->date
  EMIT CHANGES;

-- Query: Most common card types
CREATE TABLE card_types AS
  SELECT
    card->type AS card_type,
    COUNT(*) AS count
  FROM cards_stream
  GROUP BY card->type
  EMIT CHANGES;

-- Query: Cards by color identity
CREATE TABLE color_distribution AS
  SELECT
    CASE 
      WHEN ARRAY_LENGTH(card->colorIdentity) = 0 THEN 'Colorless'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 1 THEN 'Monocolored'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 2 THEN 'Two-colored'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 3 THEN 'Three-colored'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 4 THEN 'Four-colored'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 5 THEN 'Five-colored'
      ELSE 'Unknown'
    END AS color_category,
    COUNT(*) AS card_count
  FROM cards_stream
  GROUP BY CASE 
      WHEN ARRAY_LENGTH(card->colorIdentity) = 0 THEN 'Colorless'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 1 THEN 'Monocolored'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 2 THEN 'Two-colored'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 3 THEN 'Three-colored'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 4 THEN 'Four-colored'
      WHEN ARRAY_LENGTH(card->colorIdentity) = 5 THEN 'Five-colored'
      ELSE 'Unknown'
    END
  EMIT CHANGES;

-- Query: Real-time price alerts for high-value cards
CREATE STREAM price_alerts AS
  SELECT
    card_uuid,
    card_name,
    price,
    price_date,
    CONCAT('ALERT: ', card_name, ' is priced at $', CAST(price AS STRING)) AS alert_message
  FROM enriched_prices
  WHERE price > 500
  EMIT CHANGES;