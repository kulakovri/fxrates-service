package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"fxrates-service/internal/application"
	"fxrates-service/internal/domain"
	"fxrates-service/internal/infrastructure/http/openapi"
)

type Server struct {
	svc *application.FXRatesService
}

func NewServer(svc *application.FXRatesService) *Server { return &Server{svc: svc} }

func (s *Server) RequestQuoteUpdate(w http.ResponseWriter, r *http.Request, params openapi.RequestQuoteUpdateParams) {
	var body openapi.QuoteUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Pair == "" {
		writeError(w, http.StatusBadRequest, "pair is required")
		return
	}
	rePair := regexp.MustCompile(`^[A-Z]{3}/[A-Z]{3}$`)
	if !rePair.MatchString(body.Pair) {
		writeError(w, http.StatusBadRequest, "invalid pair format (e.g. EUR/USD)")
		return
	}
	id, err := s.svc.RequestQuoteUpdate(r.Context(), body.Pair, params.XIdempotencyKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp := openapi.QuoteUpdateResponse{UpdateId: id}
	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Server) GetQuoteUpdate(w http.ResponseWriter, r *http.Request, id string) {
	upd, err := s.svc.GetQuoteUpdate(r.Context(), id)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
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
	rePair := regexp.MustCompile(`^[A-Z]{3}/[A-Z]{3}$`)
	if !rePair.MatchString(params.Pair) {
		writeError(w, http.StatusBadRequest, "invalid pair format (e.g. EUR/USD)")
		return
	}
	q, err := s.svc.GetLastQuote(r.Context(), params.Pair)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
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

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(apiError{Code: code, Message: msg})
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
