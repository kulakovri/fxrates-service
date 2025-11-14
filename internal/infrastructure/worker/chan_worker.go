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
	svc  *application.FXRatesService
	jobs <-chan UpdateMsg
}

func NewChanWorker(svc *application.FXRatesService, jobs <-chan UpdateMsg) *ChanWorker {
	return &ChanWorker{svc: svc, jobs: jobs}
}

func (w *ChanWorker) Start(ctx context.Context) {
	log := logx.L().With(zap.String("worker", "chan"))
	for {
		select {
		case <-ctx.Done():
			log.Info("chan_worker.stop")
			return
		case m, ok := <-w.jobs:
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
			_ = w.svc.CompleteQuoteUpdate(ctx, m.ID, func(context.Context) (domain.Quote, error) {
				return domain.Quote{}, fmt.Errorf("panic: %s", msg)
			}, "chan")
		}
	}()
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = w.svc.CompleteQuoteUpdate(c, m.ID, func(cx context.Context) (domain.Quote, error) {
		return w.svc.FetchQuote(cx, m.Pair)
	}, "chan")
}
