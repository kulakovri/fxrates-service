CREATE TABLE IF NOT EXISTS quotes (
  pair TEXT PRIMARY KEY,
  price NUMERIC(18,6) NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS quote_updates (
  id UUID PRIMARY KEY,
  pair TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('queued','processing','done','failed')),
  error TEXT,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  idempotency_key TEXT UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_quote_updates_status ON quote_updates(status);
CREATE INDEX IF NOT EXISTS idx_quote_updates_pair ON quote_updates(pair);


