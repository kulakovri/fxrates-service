package worker

import (
	"context"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"go.uber.org/zap"
)

var _ application.Worker = (*DbWorker)(nil)

type DbWorker struct {
	Jobs     application.UpdateJobRepo
	Quotes   application.QuoteRepo
	Provider application.RateProvider

	PollEvery  time.Duration
	BatchLimit int
	Log        *zap.Logger
}

func (w *DbWorker) Start(ctx context.Context) {
	log := w.Log
	if log == nil {
		log = zap.NewNop()
	}
	if w.PollEvery <= 0 {
		w.PollEvery = 250 * time.Millisecond
	}
	if w.BatchLimit <= 0 {
		w.BatchLimit = 10
	}

	t := time.NewTicker(w.PollEvery)
	defer t.Stop()

	log.Info("db_worker_started", zap.Duration("poll_every", w.PollEvery))
	for {
		select {
		case <-ctx.Done():
			log.Info("db_worker_stopped")
			return
		case <-t.C:
			w.tick(ctx, log)
		}
	}
}

func (w *DbWorker) tick(ctx context.Context, log *zap.Logger) {
	jobs, err := w.Jobs.ClaimQueued(ctx, w.BatchLimit)
	if err != nil {
		log.Warn("claim_failed", zap.Error(err))
		return
	}
	for _, j := range jobs {
		w.processOne(ctx, log, j.ID, j.Pair)
	}
}

func (w *DbWorker) processOne(ctx context.Context, log *zap.Logger, id, pair string) {
	quote, err := w.Provider.Get(ctx, pair)
	if err != nil {
		msg := err.Error()
		_ = w.Jobs.UpdateStatus(ctx, id, domain.QuoteUpdateStatusFailed, &msg)
		log.Warn("update_failed", zap.String("id", id), zap.String("pair", pair), zap.Error(err))
		return
	}

	_ = w.Quotes.AppendHistory(ctx, domain.QuoteHistory{
		Pair:     quote.Pair,
		Price:    quote.Price,
		QuotedAt: quote.UpdatedAt,
		Source:   "provider",
		UpdateID: &id,
	})
	_ = w.Quotes.Upsert(ctx, domain.Quote{
		Pair:      quote.Pair,
		Price:     quote.Price,
		UpdatedAt: quote.UpdatedAt,
	})
	_ = w.Jobs.UpdateStatus(ctx, id, domain.QuoteUpdateStatusDone, nil)

	log.Info("update_done", zap.String("id", id), zap.String("pair", pair))
}
