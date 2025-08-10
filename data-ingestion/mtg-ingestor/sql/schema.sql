-- Create MTG database schema
CREATE DATABASE IF NOT EXISTS mtg;

-- Use the MTG database
\c mtg;

-- Create cards table
CREATE TABLE IF NOT EXISTS cards (
    uuid VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    mana_cost VARCHAR(50),
    converted_mana_cost DECIMAL(5,2),
    type VARCHAR(255),
    text TEXT,
    power VARCHAR(10),
    toughness VARCHAR(10),
    colors TEXT[], -- Array of colors
    color_identity TEXT[], -- Array of color identity
    set_code VARCHAR(10),
    rarity VARCHAR(20),
    artist VARCHAR(255),
    number VARCHAR(10),
    layout VARCHAR(50),
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create sets table
CREATE TABLE IF NOT EXISTS sets (
    code VARCHAR(10) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50),
    release_date DATE,
    base_set_size INTEGER,
    total_set_size INTEGER,
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create prices table
CREATE TABLE IF NOT EXISTS card_prices (
    id SERIAL PRIMARY KEY,
    card_uuid VARCHAR(36) REFERENCES cards(uuid),
    source VARCHAR(50), -- tcgplayer, cardmarket, etc.
    price_type VARCHAR(20), -- normal, foil
    price DECIMAL(10,2),
    currency VARCHAR(3),
    date DATE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(card_uuid, source, price_type, date)
);

-- Create legalities table
CREATE TABLE IF NOT EXISTS card_legalities (
    card_uuid VARCHAR(36) REFERENCES cards(uuid),
    format VARCHAR(50),
    legality VARCHAR(20),
    PRIMARY KEY (card_uuid, format)
);

-- Create subtypes table
CREATE TABLE IF NOT EXISTS card_subtypes (
    card_uuid VARCHAR(36) REFERENCES cards(uuid),
    subtype VARCHAR(50),
    PRIMARY KEY (card_uuid, subtype)
);

-- Create supertypes table
CREATE TABLE IF NOT EXISTS card_supertypes (
    card_uuid VARCHAR(36) REFERENCES cards(uuid),
    supertype VARCHAR(50),
    PRIMARY KEY (card_uuid, supertype)
);

-- Create keywords table
CREATE TABLE IF NOT EXISTS card_keywords (
    card_uuid VARCHAR(36) REFERENCES cards(uuid),
    keyword VARCHAR(50),
    PRIMARY KEY (card_uuid, keyword)
);

-- Create deck table for deck management
CREATE TABLE IF NOT EXISTS decks (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    format VARCHAR(50),
    description TEXT,
    created_by VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create deck_cards table for deck composition
CREATE TABLE IF NOT EXISTS deck_cards (
    deck_id INTEGER REFERENCES decks(id) ON DELETE CASCADE,
    card_uuid VARCHAR(36) REFERENCES cards(uuid),
    quantity INTEGER NOT NULL DEFAULT 1,
    is_commander BOOLEAN DEFAULT FALSE,
    is_sideboard BOOLEAN DEFAULT FALSE,
    PRIMARY KEY (deck_id, card_uuid, is_sideboard)
);

-- Create statistics table for aggregated data
CREATE TABLE IF NOT EXISTS card_statistics (
    id SERIAL PRIMARY KEY,
    set_code VARCHAR(10),
    stat_date DATE,
    total_cards INTEGER,
    mythic_count INTEGER,
    rare_count INTEGER,
    uncommon_count INTEGER,
    common_count INTEGER,
    avg_converted_mana_cost DECIMAL(5,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better query performance
CREATE INDEX idx_cards_name ON cards(name);
CREATE INDEX idx_cards_set_code ON cards(set_code);
CREATE INDEX idx_cards_rarity ON cards(rarity);
CREATE INDEX idx_cards_type ON cards(type);
CREATE INDEX idx_cards_mana_cost ON cards(converted_mana_cost);
CREATE INDEX idx_cards_colors ON cards USING GIN(colors);
CREATE INDEX idx_cards_color_identity ON cards USING GIN(color_identity);

CREATE INDEX idx_prices_card_uuid ON card_prices(card_uuid);
CREATE INDEX idx_prices_date ON card_prices(date);
CREATE INDEX idx_prices_source ON card_prices(source);

CREATE INDEX idx_legalities_format ON card_legalities(format);
CREATE INDEX idx_legalities_legality ON card_legalities(legality);

CREATE INDEX idx_decks_created_by ON decks(created_by);
CREATE INDEX idx_deck_cards_deck_id ON deck_cards(deck_id);

-- Create triggers for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_cards_updated_at BEFORE UPDATE ON cards
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sets_updated_at BEFORE UPDATE ON sets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_decks_updated_at BEFORE UPDATE ON decks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create views for common queries
CREATE OR REPLACE VIEW v_card_details AS
SELECT 
    c.uuid,
    c.name,
    c.mana_cost,
    c.converted_mana_cost,
    c.type,
    c.text,
    c.power,
    c.toughness,
    c.colors,
    c.color_identity,
    c.set_code,
    s.name as set_name,
    c.rarity,
    c.artist,
    c.number
FROM cards c
LEFT JOIN sets s ON c.set_code = s.code;

CREATE OR REPLACE VIEW v_deck_composition AS
SELECT 
    d.id as deck_id,
    d.name as deck_name,
    d.format,
    c.name as card_name,
    dc.quantity,
    dc.is_commander,
    dc.is_sideboard,
    c.mana_cost,
    c.converted_mana_cost,
    c.type,
    c.rarity
FROM decks d
JOIN deck_cards dc ON d.id = dc.deck_id
JOIN cards c ON dc.card_uuid = c.uuid
ORDER BY d.id, dc.is_sideboard, c.converted_mana_cost, c.name;

-- Grant permissions (adjust as needed)
GRANT ALL PRIVILEGES ON DATABASE mtg TO mtg_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO mtg_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO mtg_user;