ALTER TABLE IF EXISTS quote_updates
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT UNIQUE;


