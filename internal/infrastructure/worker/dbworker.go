package worker

import (
	"context"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/config"

	"go.uber.org/zap"
)

var _ application.Worker = (*DbWorker)(nil)

type DbWorker struct {
	svc *application.FXRatesService

	PollEvery  time.Duration
	BatchLimit int
	Log        *zap.Logger
}

func NewDBWorker(svc *application.FXRatesService, pollEvery time.Duration, batchLimit int, log *zap.Logger) *DbWorker {
	return &DbWorker{
		svc:        svc,
		PollEvery:  pollEvery,
		BatchLimit: batchLimit,
		Log:        log,
	}
}

func (w *DbWorker) Start(ctx context.Context) {
	log := w.Log
	if log == nil {
		log = zap.NewNop()
	}
	if w.PollEvery <= 0 {
		w.PollEvery = config.Load().WorkerPoll
	}
	if w.BatchLimit <= 0 {
		w.BatchLimit = config.Load().WorkerBatchSize
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
	if err := w.svc.ProcessQueueBatch(ctx, w.BatchLimit); err != nil {
		log.Warn("batch_error", zap.Error(err))
	}
}
