package httpserver

import (
	"fxrates-service/internal/domain"
	openapi "fxrates-service/internal/infrastructure/http/openapi"
	"testing"
)

func Test_mapStatus(t *testing.T) {
	cases := []struct {
		in  domain.QuoteUpdateStatus
		out openapi.QuoteUpdateDetailsStatus
	}{
		{domain.QuoteUpdateStatusQueued, openapi.Pending},
		{domain.QuoteUpdateStatusProcessing, openapi.Pending},
		{domain.QuoteUpdateStatusDone, openapi.Completed},
		{domain.QuoteUpdateStatusFailed, openapi.Failed},
	}
	for _, c := range cases {
		got := mapStatus(c.in)
		if got != c.out {
			t.Fatalf("mapStatus(%v)=%v want %v", c.in, got, c.out)
		}
	}
}
