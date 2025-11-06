package worker

import (
	"context"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
)

type InMemWorker struct {
	Updates   application.UpdateJobRepo
	Quotes    application.QuoteRepo
	Provider  application.RateProvider
	PollEvery time.Duration
}

type queuedLister interface{ ListQueuedIDs() []string }

func queuedIDs(repo application.UpdateJobRepo) []string {
	if l, ok := repo.(queuedLister); ok {
		return l.ListQueuedIDs()
	}
	return nil
}

func (w *InMemWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.PollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids := queuedIDs(w.Updates)
			for _, id := range ids {
				w.processJob(ctx, id)
			}
		}
	}
}

func (w *InMemWorker) processJob(ctx context.Context, id string) {
	job, err := w.Updates.GetByID(ctx, id)
	if err != nil {
		return
	}

	_ = w.Updates.UpdateStatus(ctx, id, domain.QuoteUpdateStatusProcessing, nil)

	q, err := w.Provider.Get(ctx, string(job.Pair))
	if err != nil {
		msg := err.Error()
		_ = w.Updates.UpdateStatus(ctx, id, domain.QuoteUpdateStatusFailed, &msg)
		return
	}

	_ = w.Quotes.Upsert(ctx, q)
	_ = w.Updates.UpdateStatus(ctx, id, domain.QuoteUpdateStatusDone, nil)
}
