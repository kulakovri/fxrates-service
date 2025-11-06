package domain

type QuoteUpdateStatus string

const (
	QuoteUpdateStatusQueued     QuoteUpdateStatus = "queued"
	QuoteUpdateStatusProcessing QuoteUpdateStatus = "processing"
	QuoteUpdateStatusDone       QuoteUpdateStatus = "done"
	QuoteUpdateStatusFailed     QuoteUpdateStatus = "failed"
)
