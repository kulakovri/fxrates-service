# Design Decisions

## Quotes history (immutable)

Introduced `quotes_history` as an immutable, append-only event log for every fetched FX quote.
`quotes` remains a mutable, last-known state table for fast lookups.
This enables temporal analysis and replay while keeping read paths simple.
All database tables use plural names for consistency.


