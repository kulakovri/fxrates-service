package worker

import (
	"context"
	"fmt"
	"time"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/logx"

	"go.uber.org/zap"
)

type UpdateMsg struct {
	ID      string
	Pair    string
	TraceID string
}

type ChanWorker struct {
	Jobs     <-chan UpdateMsg
	Provider application.RateProvider
	Quotes   application.QuoteRepo
	JobsRepo application.UpdateJobRepo
}

func (w *ChanWorker) Start(ctx context.Context) {
	log := logx.L().With(zap.String("worker", "chan"))
	for {
		select {
		case <-ctx.Done():
			log.Info("chan_worker.stop")
			return
		case m, ok := <-w.Jobs:
			if !ok {
				log.Info("chan_worker.closed")
				return
			}
			w.processOne(ctx, m)
		}
	}
}

func (w *ChanWorker) processOne(ctx context.Context, m UpdateMsg) {
	defer func() {
		if r := recover(); r != nil {
			logx.L().Warn("chan_worker.panic", zap.Any("r", r))
			msg := fmt.Sprint(r)
			_ = w.JobsRepo.UpdateStatus(ctx, m.ID, domain.QuoteUpdateStatusFailed, &msg)
		}
	}()
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	log := logx.L().With(
		zap.String("update_id", m.ID),
		zap.String("pair", m.Pair),
		zap.String("trace_id", m.TraceID),
	)
	// Optional: mark processing for visibility
	_ = w.JobsRepo.UpdateStatus(c, m.ID, domain.QuoteUpdateStatusProcessing, nil)

	q, err := w.Provider.Get(c, m.Pair)
	if err != nil {
		msg := err.Error()
		_ = w.JobsRepo.UpdateStatus(c, m.ID, domain.QuoteUpdateStatusFailed, &msg)
		log.Warn("chan_worker.fetch_failed", zap.Error(err))
		return
	}

	_ = w.Quotes.AppendHistory(c, domain.QuoteHistory{
		Pair:     q.Pair,
		Price:    q.Price,
		QuotedAt: q.UpdatedAt,
		Source:   "chan",
		UpdateID: &m.ID,
	})
	_ = w.Quotes.Upsert(c, q)

	now := time.Now().UTC()
	_ = w.JobsRepo.UpdateStatus(c, m.ID, domain.QuoteUpdateStatusDone, nil)
	log.Info("chan_worker.done", zap.Time("updated_at", now))
}
