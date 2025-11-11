package config

import "time"

const (
	DefaultHTTPPort        = "8080"
	DefaultShutdownTimeout = 10 * time.Second
	DefaultWorkerPoll      = 250 * time.Millisecond
	DefaultWorkerBatch     = 10
	DefaultPGMaxConns      = 5
	DefaultPGMinConns      = 1
)
