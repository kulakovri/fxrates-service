CREATE TABLE IF NOT EXISTS quotes_history (
  id           BIGSERIAL PRIMARY KEY,
  pair         TEXT          NOT NULL,
  price        NUMERIC(18,6) NOT NULL,
  quoted_at    TIMESTAMPTZ   NOT NULL,
  source       TEXT          DEFAULT 'worker',
  update_id    UUID REFERENCES quote_updates(id) ON DELETE SET NULL,
  inserted_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
  UNIQUE (pair, quoted_at, source)
);

CREATE INDEX IF NOT EXISTS idx_quotes_history_pair_time
  ON quotes_history (pair, quoted_at DESC);


