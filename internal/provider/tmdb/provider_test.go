package tmdb

import (
	"errors"
	"testing"

	"github.com/Digital-Shane/title-tidy/internal/provider"
)

func TestMapError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "nil"},
		{name: "missing resource", err: errors.New("Code (34): The resource you requested could not be found."), code: "NOT_FOUND"},
		{name: "invalid ID", err: errors.New("Code (6): Invalid id: The pre-requisite id is invalid or not found."), code: "NOT_FOUND"},
		{name: "HTTP not found", err: errors.New("404 Not Found"), code: "NOT_FOUND"},
		{name: "authentication", err: errors.New("401 Unauthorized"), code: "AUTH_FAILED"},
		{name: "rate limit", err: errors.New("429 Too Many Requests"), code: "RATE_LIMITED"},
		{name: "unavailable", err: errors.New("503 Service Unavailable"), code: "UNAVAILABLE"},
		{name: "missing session", err: errors.New("Code (37): The requested session could not be found."), code: "UNKNOWN"},
		{name: "network failure", err: errors.New("host not found"), code: "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New().mapError(tt.err)
			if tt.err == nil {
				if err != nil {
					t.Fatalf("mapError(nil) = %v", err)
				}
				return
			}
			var provErr *provider.ProviderError
			if !errors.As(err, &provErr) || provErr.Code != tt.code {
				t.Fatalf("mapError(%v) = %v, want code %s", tt.err, err, tt.code)
			}
			if tt.code == "NOT_FOUND" && provErr.Retry {
				t.Error("missing resources should require manual correction, not automatic retries")
			}
		})
	}
}
