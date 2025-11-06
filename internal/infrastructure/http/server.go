package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/http/openapi"
)

var ErrNotFound = errors.New("not found")

type Server struct {
	svc *application.FXRatesService
}

func NewServer(svc *application.FXRatesService) *Server { return &Server{svc: svc} }

func (s *Server) RequestQuoteUpdate(w http.ResponseWriter, r *http.Request, params openapi.RequestQuoteUpdateParams) {
	var body openapi.QuoteUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	if body.Pair == "" {
		badRequest(w, "pair is required")
		return
	}
	id, err := s.svc.RequestQuoteUpdate(r.Context(), body.Pair, params.XIdempotencyKey)
	if err != nil {
		internalError(w)
		return
	}
	resp := openapi.QuoteUpdateResponse{UpdateId: id}
	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Server) GetQuoteUpdate(w http.ResponseWriter, r *http.Request, id string) {
	upd, err := s.svc.GetQuoteUpdate(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			notFound(w)
			return
		}
		internalError(w)
		return
	}
	resp := openapi.QuoteUpdateDetails{
		UpdateId:  upd.ID,
		Pair:      string(upd.Pair),
		Status:    mapStatus(upd.Status),
		UpdatedAt: upd.UpdatedAt,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) GetLastQuote(w http.ResponseWriter, r *http.Request, params openapi.GetLastQuoteParams) {
	q, err := s.svc.GetLastQuote(r.Context(), params.Pair)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			notFound(w)
			return
		}
		internalError(w)
		return
	}
	var price *float32
	if q.Price != 0 {
		p := float32(q.Price)
		price = &p
	}
	resp := openapi.LastQuote{
		Pair:      string(q.Pair),
		Price:     price,
		UpdatedAt: q.UpdatedAt,
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func badRequest(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

func notFound(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func internalError(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func mapStatus(s domain.QuoteUpdateStatus) openapi.QuoteUpdateDetailsStatus {
	switch s {
	case domain.QuoteUpdateStatusDone:
		return openapi.Completed
	case domain.QuoteUpdateStatusFailed:
		return openapi.Failed
	default:
		return openapi.Pending
	}
}
